1. Yêu cầu:
News Feed:
- Đăng bài, có hiển thị số lượng like, không cần real time, dùng cơ chế pull-to-refresh 
- Theo dõi người dùng (1 chiều, không cần kết bạn)
- Xem feed, sắp xếp theo thời gian feed của người mà user follow

Messaging:
- Chat 1-1, group chat deep dive sau
- Real-time: tin nhắn phải được gửi và nhận tức thời khi cả hai cùng online
- Trạng thái tin nhắn: đang gửi, đã gửi, gửi không được, đã đọc
- Tin nhắn text, media
- Offline Notification: gửi thông báo cho người dùng 


2. Yêu cầu phi chức năng:
- DAU: 100 triệu
- 1% đăng bài, mỗi ngày trung bình 1 bài
- Trung bình lướt Feed 10 lần/ngày
- Mỗi người dùng trung bình 200 người
- 50% sử dụng tính năng chat
- Một người trung bình nhắn 50 tin 


3. Tính toán:
- QPS trung bình cho New Feed: 
	+ Đăng bài: Trung bình có thêm 1,000,000 bài đăng mới mỗi ngày -> QPS = 12 -> Peak = 60
	+ Đọc Feed: 100,000,000 * 10 = 1 tỉ request mỗi ngày -> QPS = 12,000 -> Peak = 60,000
- QPS trung bình cho việc đọc và gửi tin nhắn:
	+ Gửi tin: 100,000,000 * 50% * 50 = 2,500,000,000 tin nhắn mỗi ngày -> QPS = 30,000 -> Peak = 150,000
	+ Tải lịch sử tin nhắn: 50% * 100,000,000 * 10 = 1 tỉ request mỗi ngày -> QPS = 6,000 -> Peak = 30,000
	+ Số lượng socket đồng thời: 5 triệu
	+ Số event peak: 600,000 events/s -> lúc bình thường: 120,000 events/s 	
- Dung lượng lưu trữ:
	+ Trung bình 1 ngày: 200GB data lưu feed + 250GB lưu message -> 500 GB data mỗi ngày


4. API Design:
 a. Đăng bài viết mới:
  - POST news/presigns -> 
	Body: [media_id,...]
	Response: [{media_id, presign_url},...]
	-> Get list presign_url for each media

  - POST news/
    	Body: - content
	      - [media_url,...]
	Response: feed_id, ...

 b. Tải Feed:
  - GET news/?cursor=&size=
	Param: cursor: last_created_feed 
 

 c. Gửi tin nhắn:
  - Dùng WebSocket 
 

 d. Tải lịch sử chat: 
  - GET chats/?cursor=&size=



5. Model:
Chọn NoSQL để lưu dữ liệu vì có tốc độ đọc ghi tốt (còn SQL có đọc ghi tốt hay không thì tôi chưa biết)
- User: id, name, email, password_hash, created_at, updated_at
	+ Index: id, email
- User_Follow: user_follow, user_target
	+ Index: (user_follow, user_target)
- Post: id, content, urls_media, created_at, updated_at, user_like_ids
	+ Index: id , (id,created_at)
- Message: id, conversation_id, sender_id, content_type, content, url_media, content, created_at, updated_at
	+ Index: id





CHI TIẾT KIẾN TRÚC TỔNG QUAN (HIGH-LEVEL DESIGN)
Hệ thống được thiết kế theo mô hình Microservices kết hợp Kiến trúc hướng sự kiện (Event-Driven Architecture), chia tách hoàn toàn giữa hai nhóm tài nguyên:
Stateless Tier (Tầng không lưu trạng thái): Xử lý các yêu cầu HTTP truyền thống (Feed, Post, User Profile). Tầng này dễ dàng scale bằng cách tăng số lượng replica phía sau Load Balancer.
Stateful Tier (Tầng lưu trạng thái): Quản lý kết nối WebSocket của hàng triệu người dùng trực tuyến. Tầng này cần các cơ chế định tuyến và theo dõi session đặc thù.
I. MÔ TẢ CHI TIẾT CÁC THÀNH PHẦN (COMPONENT BREAKDOWN)
1. Tầng Gateway & Routing
API Gateway (HTTP): Điểm đầu vào cho mọi yêu cầu REST API từ Client. Nhiệm vụ: Xác thực người dùng (JWT/Session Authentication), Giới hạn tần suất gọi (Rate Limiting), Định tuyến yêu cầu (Routing) đến đúng microservice phía sau.
WS Gateway (Cụm WebSocket Nodes): Cụm máy chủ chuyên dụng để duy trì các kết nối TCP/WebSocket lâu dài với Client. Tầng này siêu nhẹ, không chứa logic nghiệp vụ nặng hay truy vấn DB, nhiệm vụ chính là làm "đầu cầu" đẩy dữ liệu real-time xuống Client.
2. Nhóm Stateless Microservices (Tầng HTTP)
User/Follow Service: Quản lý thông tin người dùng và đồ thị follow (ai follow ai). Dịch vụ này tương tác trực tiếp với Postgres DB.
Post Service: Tiếp nhận yêu cầu đăng bài, tải ảnh lên S3/CDN và lưu metadata của bài viết vào MongoDB. Khi tạo post thành công, nó sẽ phát (publish) một sự kiện sang Kafka.
Feed Service: Chịu trách nhiệm tổng hợp bài viết để trả về News Feed cho người dùng. Nó ưu tiên đọc từ bộ nhớ đệm Redis Feed Cache, nếu trống sẽ fallback xuống MongoDB và Postgres.
3. Nhóm Stateful Services & Workers (Tầng Real-time)
Presence Service: Quản lý trạng thái online/offline của người dùng. Nó lưu trữ một bảng ánh xạ nhanh trong Redis Session Store để biết: User_ID nào đang online ở WS_Node_IP nào.
Chat Engine (Consumer): Một dịch vụ chạy nền tiêu thụ (consume) tin nhắn từ Kafka. Nó thực hiện các tác vụ nặng: Ghi tin nhắn vào MongoDB, kiểm tra block, kiểm tra online status và chuyển hướng tin nhắn.
Fan-out Worker: Worker chạy nền tiêu thụ sự kiện PostCreated từ Kafka để phân phối bài viết đến Feed của các follower.
Push Notification Service: Tích hợp với Apple Push Notification (APNs) và Firebase Cloud Messaging (FCM) để gửi thông báo màn hình khóa cho người dùng offline.
4. Tầng Lưu Trữ Dữ Liệu & Event Streaming
PostgreSQL (User & Follow DB): Lưu trữ dữ liệu có tính quan hệ cao, cần giao dịch ACID.
MongoDB (Post DB): Lưu trữ dữ liệu bài đăng dưới dạng JSON document linh hoạt.
MongoDB (Chat DB): Chuyên dụng cho việc ghi dữ liệu tin nhắn siêu nhanh với quy mô hàng tỷ bản ghi, tối ưu hóa truy vấn sắp xếp theo thời gian.
Redis Cluster: Chia làm 2 cụm độc lập:
Cụm 1 (Presence & Session): Lưu trạng thái online và mapping IP kết nối.
Cụm 2 (Feed Cache): Lưu trữ các Sorted Set (ZSET) chứa danh sách ID bài viết của News Feed cho từng người dùng.
Kafka (Event Stream & Message Queue): Trục xương sống giúp bất đồng bộ hóa và giảm tải cho hệ thống. Nó chứa các topic như post-created (cho luồng feed), chat-incoming (cho luồng gửi tin).
II. CHI TIẾT 4 LUỒNG DỮ LIỆU CHÍNH (DATA FLOWS)
1. Luồng Đăng Bài và Phân Phối Feed (Fan-out on Write)
code
Code
[Client] ──(1. POST /posts)──► [Post Service] ──(2. Save)──► [MongoDB]
                                    │
                            (3. Push Event)
                                    │
                                    ▼
                              [Kafka Topic]
                                    │
                            (4. Consume Event)
                                    │
                                    ▼
                            [Fan-out Worker]
                                    │
                       (5. Query Followers) ──► [Postgres (Follows)]
                                    │
                  (6. Push Post_ID to Redis ZSET)
                                    │
                                    ▼
                            [Redis Feed Cache]
Client gửi yêu cầu đăng bài kèm nội dung và link ảnh tới Post Service thông qua HTTP.
Post Service lưu dữ liệu bài đăng vào MongoDB.
Post Service đẩy sự kiện PostCreated {post_id, author_id, timestamp} vào Kafka Topic post-created.
Fan-out Worker tiêu thụ sự kiện này từ Kafka.
Worker truy vấn Postgres để lấy danh sách tất cả những người đang follow author_id.
Xử lý Hybrid:
Nếu tác giả là người bình thường (< 10,000 followers), Worker sẽ lặp qua danh sách follower và đẩy post_id vào Redis Feed Cache (ZSET) của từng follower: ZADD feed:user:<follower_id> <timestamp> <post_id>.
Nếu tác giả là người nổi tiếng (Celebrity), Worker bỏ qua bước này để tránh làm quá tải Redis (hiện tượng write amplification).
2. Luồng Tải Feed (Read Feed - Hybrid Pull/Push)
code
Code
[Client] ──(1. GET /feeds)──► [Feed Service]
                                    │
                   ┌────────────────┴────────────────┐
          (2a. Đọc Cache)                   (2b. Đọc Người nổi tiếng)
                   ▼                                 ▼
           [Redis Feed Cache]                     [MongoDB]
             (Lấy Post_IDs)                  (Lấy Posts của Idol)
                   │                                 │
                   └────────────────┬────────────────┘
                                    ▼
                           (3. Gộp & Sắp xếp)
                                    │
                        (4. Truy vấn nội dung chi tiết)
                                    │
                                    ▼
                            [Client nhận Feed]
Client gửi yêu cầu lấy News Feed bằng phương thức HTTP GET kèm tham số phân trang cursor (timestamp của bài viết cũ nhất của trang trước).
Feed Service thực hiện song song hai việc:
2a. Đọc từ Redis Feed Cache (ZSET) của người dùng để lấy ra danh sách 10 đến 20 post_id gần nhất khớp với khoảng thời gian cursor.
2b. Truy vấn danh sách những người nổi tiếng mà người dùng này đang follow (lấy từ cache hoặc Postgres), sau đó kéo trực tiếp các bài viết mới của những người nổi tiếng này từ MongoDB (Pull Model).
Feed Service thực hiện gộp (merge) danh sách bài viết từ Redis Cache và bài viết từ người nổi tiếng, sắp xếp lại theo thời gian giảm dần.
Service truy vấn nội dung chi tiết (content, media_urls, author_info) của các post_id này từ MongoDB (hoặc Post Document Cache) rồi trả về cho Client.
3. Luồng Gửi & Nhận Tin Nhắn Real-time (Chat 1-1)
code
Code
[User A] ──(1. WS Frame: CHAT_SEND)──► [WS Node 1] ──(2. Push)──► [Kafka (Incoming)]
                                                                        │
                                                                 (3. Consume)
                                                                        │
                                                                        ▼
                                                                 [Chat Engine]
                                                                        │
                                                    ┌───────────────────┴───────────────────┐
                                          (4a. Write DB)                       (4b. Tra cứu Online)
                                                    ▼                                       ▼
                                               [MongoDB]                          [Presence Redis]
                                                                                            │
                                                                            ┌───────────────┴───────────────┐
                                                                     (Nếu Online)                     (Nếu Offline)
                                                                            ▼                               ▼
                                                                      [WS Node 2]                     [Push Service]
                                                                            │                               │
                                                                            ▼                               ▼
                                                                     [User B nhận WS]                [Notification]
User A gửi tin nhắn dưới dạng gói tin WebSocket CHAT_SEND đến máy chủ WS Node 1 (nơi họ đang giữ kết nối).
WS Node 1 nhận tin, ngay lập tức đẩy tin nhắn vào Kafka Topic chat-incoming để xử lý bất đồng bộ, đồng thời gửi lại một gói tin CHAT_ACK cho User A báo "Server đã nhận tin".
Chat Engine tiêu thụ tin nhắn từ Kafka.
Dịch vụ này thực hiện hai tác vụ song song:
4a. Ghi tin nhắn vào database lưu trữ MongoDB.
4b. Truy vấn Presence Redis để kiểm tra trạng thái của User B.
Định tuyến dựa trên trạng thái:
Nếu User B đang Online (ví dụ đang ở WS Node 2): Chat Engine gửi tin nhắn đến WS Node 2 thông qua cơ chế nội bộ (gRPC hoặc Redis Pub/Sub). WS Node 2 tìm trong bộ nhớ kết nối TCP của User B và đẩy gói tin CHAT_RECEIVE xuống thiết bị của User B.
Nếu User B Offline: Chat Engine gọi sang Push Notification Service để gửi thông báo đẩy qua Firebase/APNs.
4. Luồng Trạng Thái Trực Tuyến (Presence & Heartbeat)
Đăng nhập (Connect): Khi Client kết nối thành công tới một WS Node, Node đó sẽ lưu trạng thái của user vào Redis: SET presence:user:<user_id> "<node_ip>" EX 60 [1].
Duy trì trạng thái (Heartbeat): Định kỳ mỗi 20 giây, Client phải gửi một gói tin PING siêu nhẹ lên WS Node. Khi nhận được PING, WS Node sẽ tự động làm mới (refresh) thời gian hết hạn (TTL) của key trong Redis thêm 60 giây [1].
Đứt kết nối (Disconnect/Timeout): Nếu Client chủ động đóng tab hoặc bị mất mạng đột ngột:
Nếu đóng tab chủ động: WS Node sẽ xóa ngay key trong Redis [1].
Nếu mất mạng đột ngột: Sau 60 giây không nhận được PING, key trong Redis sẽ tự động hết hạn (expire) [1]. Hệ thống tự hiểu người dùng đã offline mà không cần can thiệp thủ công [1].
III. CÁC QUYẾT ĐỊNH THIẾT KẾ THEN CHỐT & ĐÁNH ĐỔI (TRADE-OFFS)
Chọn Hybrid Model (Push + Pull) cho News Feed:
Tại sao: Nếu dùng thuần Push, khi người nổi tiếng đăng bài, hệ thống phải ghi dữ liệu vào hàng triệu cache của followers, gây nghẽn Redis cực hạn. Nếu dùng thuần Pull, mỗi lần người dùng lướt feed sẽ phải chạy các câu query phức tạp vào DB để tìm bài viết từ bạn bè, gây quá tải DB khi QPS đọc lớn (60,000 QPS).
Đánh đổi: Sự phức tạp của code tăng lên vì phải quản lý cả 2 luồng đọc/ghi song song và thực hiện gộp dữ liệu ở tầng ứng dụng.
Sử dụng MongoDB cho Chat DB thay vì MySQL/PostgreSQL:
Tại sao: Với 2.5 tỷ tin nhắn một ngày, relational DB truyền thống sẽ nhanh chóng bị quá tải dung lượng và tốc độ ghi. MongoDB sử dụng cấu trúc LSM-tree giúp tốc độ ghi tuần tự lên đĩa cực nhanh và hỗ trợ scale ngang không giới hạn.
Đánh đổi: Không thể thực hiện các câu truy vấn thống kê, tìm kiếm hay JOIN phức tạp trên tin nhắn. Chúng ta chấp nhận đánh đổi điều này để lấy hiệu năng ghi và khả năng lưu trữ không giới hạn.
Tách biệt Write Path qua Kafka trong Chat:
Tại sao: Chúng ta không để máy chủ WebSocket ghi trực tiếp vào MongoDB. Khi có đột biến tin nhắn hoặc DB bị nghẽn nhẹ, Kafka sẽ đóng vai trò là "bể chứa" (buffer) giảm áp lực, tin nhắn được xếp hàng để ghi từ từ vào DB mà không làm đứt kết nối hay nghẽn I/O của WebSocket Server.
Đánh đổi: Tin nhắn sẽ có một độ trễ rất nhỏ (vài phần mười giây) để lưu vào DB (Eventual Consistency), nhưng trải nghiệm người dùng vẫn được đảm bảo nhờ cơ chế gửi ACK tạm thời từ Socket Server.