# Tài liệu Thiết kế các Tính năng Nâng cao (Advanced Features Design)

Tài liệu này đặc tả thiết kế chi tiết cho các tính năng nâng cao được đề xuất để nâng cấp hệ thống mạng xã hội (social-network-system) lên quy mô lớn (production-ready). Các giải pháp tập trung vào tối ưu hóa hiệu năng, giảm tải cho cơ sở dữ liệu, đảm bảo tính thời gian thực (real-time) và khả năng giám sát hệ thống.

---

## 1. Trạng thái tin nhắn Real-time (Message Delivery & Read Status)

Hiện tại, hệ thống mới chỉ phản hồi `CHAT_ACK` để xác nhận tin nhắn đã tới WebSocket Node. Để hiển thị trạng thái đã nhận (delivered) và đã đọc (read) như các ứng dụng chat hiện đại, chúng ta thiết kế cơ chế sau:

### 1.1 Luồng Dữ liệu Trạng thái (Data Flows)

```mermaid
sequenceDiagram
    autonumber
    actor UserA as User A (Sender)
    participant WS1 as WS Node 1
    actor UserB as User B (Recipient)
    participant WS2 as WS Node 2
    participant CE as Chat Engine (Worker)
    participant DB as MongoDB (Chat DB)

    Note over UserA, UserB: Luồng 1: Delivered Status
    WS2->>UserB: WS Frame: CHAT_RECEIVE (Tin nhắn)
    UserB->>WS2: WS Frame: CHAT_DELIVERED (Client Ack)
    WS2->>CE: Push to Kafka (Topic: chat-status-updates)
    CE->>DB: Update Message Status: delivered_at = Now()
    CE->>WS1: Redis Pub/Sub: Notify User A
    WS1->>UserA: WS Frame: CHAT_STATUS_UPDATED (Delivered)

    Note over UserA, UserB: Luồng 2: Read Status
    UserB->>WS2: User B mở khung chat -> WS Frame: CHAT_READ (last_msg_id)
    WS2->>CE: Push to Kafka (Topic: chat-status-updates)
    CE->>DB: Update all messages in conversation <= last_msg_id to status = read
    CE->>WS1: Redis Pub/Sub: Notify User A
    WS1->>UserA: WS Frame: CHAT_STATUS_UPDATED (Read)
```

### 1.2 Thiết kế Schema MongoDB cập nhật
Mỗi tin nhắn sẽ cần bổ sung các trường thời gian để ghi nhận trạng thái:

```json
{
  "_id": "ObjectID",
  "conversation_id": "string",
  "sender_id": "ObjectID",
  "recipient_id": "ObjectID",
  "content_type": "string",
  "content": "string",
  "created_at": "ISODate",
  "delivered_at": "ISODate (nullable)",
  "read_at": "ISODate (nullable)"
}
```

*   **Chỉ mục (Indexes) bổ sung**:
    *   `{ "conversation_id": 1, "read_at": 1 }` để nhanh chóng truy vấn những tin nhắn chưa đọc và cập nhật trạng thái đọc hàng loạt.

---

## 2. Chat Nhóm (Group Chat)

Để hỗ trợ chat nhiều người, chúng ta cần chuyển đổi từ mô hình chat 1-1 thuần túy sang mô hình hội thoại (Conversations) trừu tượng.

### 2.1 Thiết kế Cơ sở dữ liệu (MongoDB)
*   **Collection**: `conversations`
```json
{
  "_id": "ObjectID",
  "type": "string (GROUP / DIRECT)",
  "name": "string (chỉ dùng cho GROUP)",
  "avatar_url": "string (nullable)",
  "creator_id": "ObjectID",
  "members": [
    {
      "user_id": "ObjectID",
      "role": "string (ADMIN / MEMBER)",
      "joined_at": "ISODate"
    }
  ],
  "last_message": {
    "msg_id": "ObjectID",
    "sender_id": "ObjectID",
    "content": "string",
    "created_at": "ISODate"
  },
  "created_at": "ISODate",
  "updated_at": "ISODate"
}
```

### 2.2 Luồng Phân phối Tin nhắn Nhóm (Fan-out Message)

```mermaid
graph TD
    UserA[User A] -->|1. WS: CHAT_SEND to Group X| WS1[WS Node 1]
    WS1 -->|2. Push Event| KafkaIncoming[Kafka Topic: chat-incoming]
    KafkaIncoming -->|3. Consume| ChatEngine[Chat Engine Worker]
    ChatEngine -->|4. Save Message| MongoDB[(MongoDB Chat DB)]
    ChatEngine -->|5. Query Group Members| MongoConv[(MongoDB Conversations)]
    ChatEngine -->|6. Check Presence for each Member| RedisPresence[(Redis Presence Store)]
    
    ChatEngine -->|7. Publish to WS Nodes| RedisPubSub{Redis Pub/Sub Channel}
    RedisPubSub -->|Node 1| WS1
    RedisPubSub -->|Node 2| WS2[WS Node 2]
    RedisPubSub -->|Node 3| WS3[WS Node 3]
    
    WS2 -->|8. Push WS Frame| UserB[User B - Group Member]
    WS3 -->|8. Push WS Frame| UserC[User C - Group Member]
```

*   **Tối ưu hóa Presence cho Nhóm lớn**: Với nhóm chat có hàng nghìn thành viên, việc tra cứu từng thành viên trong Redis có thể tốn kém. Giải pháp là lưu danh sách các WebSocket Node đang có ít nhất 1 thành viên của nhóm đang online (Node-level subscription). Thay vì gửi tới từng cá nhân, Chat Engine chỉ cần publish tin nhắn tới channel của các WS Node đó.

---

## 3. Trạng thái "Đang gõ..." (Typing Indicator)

Đây là tính năng real-time thuần túy, không cần lưu trữ vĩnh viễn (persistent). Do đó, chúng ta sẽ thiết kế một luồng **bất tuần tự (bypass)** hoàn toàn qua Database và Kafka để giảm thiểu độ trễ tối đa.

```mermaid
sequenceDiagram
    autonumber
    actor UserA as User A
    participant WS1 as WS Node 1
    participant Redis as Redis Pub/Sub
    participant WS2 as WS Node 2
    actor UserB as User B

    UserA->>WS1: WS Frame: TYPING { conversation_id, is_typing: true }
    Note over WS1: Tra cứu Presence của User B<br/>Biết User B đang ở WS Node 2
    WS1->>Redis: PUBLISH chat_node:WS2 { type: "TYPING", sender_id: A, conversation_id }
    Redis->>WS2: Message received
    WS2->>UserB: WS Frame: TYPING_INDICATOR { sender_id: A, conversation_id, is_typing: true }
```

*   **Cơ chế chống nhiễu (Debouncing):** Client chỉ gửi gói tin `TYPING` 1 lần mỗi 3 giây khi người dùng liên tục gõ phím. Nếu sau 5 giây không nhận được gói tin `TYPING` tiếp theo, client của người nhận sẽ tự động ẩn trạng thái đang gõ.

---

## 4. Danh sách Bạn bè Online (Presence List Real-time)

Để hiển thị danh sách người dùng đang hoạt động (Online/Offline status) cho bạn bè/follower của họ.

### 4.1 Cơ chế Đăng ký Sự kiện Presence (Presence Subscription)
Khi Client kết nối tới WebSocket Node:
1.  Hệ thống truy vấn Postgres lấy danh sách những người mà User đó đang follow (Following list).
2.  WS Node đăng ký (subscribe) vào Redis Pub/Sub channels của tất cả những người trong Following list đó: `user_presence:<target_user_id>`.
3.  Khi bất kỳ người nào trong danh sách online/offline, sự kiện thay đổi trạng thái sẽ được push trực tiếp tới WS Node để cập nhật UI của client.

### 4.2 Luồng lan truyền trạng thái (Presence Fan-out)

```mermaid
graph LR
    UserA[User A Connects] -->|1. Set Online| WS1[WS Node 1]
    WS1 -->|2. Publish Status| RedisPubSub{Redis Pub/Sub}
    RedisPubSub -->|Channel: user_presence:A| WS2[WS Node 2]
    RedisPubSub -->|Channel: user_presence:A| WS3[WS Node 3]
    WS2 -->|3. Push Update| FollowerB[Follower B of A]
    WS3 -->|3. Push Update| FollowerC[Follower C of A]
```

---

## 5. Active Cache Warm-up (Khởi động lại Cache Feed)

Redis Feed Cache lưu trữ feed dưới dạng Sorted Set (`ZSET`) với TTL là 7 ngày. Đối với người dùng ít hoạt động, cache của họ sẽ bị xoá để tiết kiệm RAM. Khi họ đăng nhập trở lại, việc truy vấn MongoDB để build lại cache có thể gây lag.

### 5.1 Giải pháp Active Warm-up qua Kafka

```mermaid
sequenceDiagram
    autonumber
    actor User as User
    participant Auth as Auth Service
    participant Redis as Redis Cache
    participant Kafka as Kafka (Topic: feed-warmup)
    participant Worker as Fan-out Worker
    participant DB as MongoDB & Postgres

    User->>Auth: POST /login
    Auth->>Redis: Check if feed:user:id exists
    alt Cache Miss (Không tồn tại)
        Auth->>Kafka: Publish Event { user_id }
        Auth-->>User: Trả về Login Successful (Không chặn)
        Worker->>Kafka: Consume Event { user_id }
        Worker->>DB: Query Followings & pull latest posts
        Worker->>Redis: ZADD feed:user:id <timestamp> <post_id> (TTL 7 ngày)
    else Cache Hit (Đã có sẵn)
        Auth-->>User: Trả về Login Successful
    end
```

---

## 6. Xử lý Media bất đồng bộ (Image/Video Transcoding Worker)

Việc bắt người dùng đợi upload ảnh dung lượng lớn hoặc video gốc trực tiếp sẽ làm giảm trải nghiệm người dùng nghiêm trọng. Thiết kế tối ưu hóa luồng tải lên và xử lý media:

### 6.1 Kiến trúc Xử lý Bất đồng bộ

```mermaid
graph TD
    Client[Client] -->|1. Request Presigned URL| PostService[Post Service]
    PostService -->|2. Return S3 Presigned URL| Client
    Client -->|3. Direct Upload Raw File| RawS3[(S3 Bucket: raw-media)]
    Client -->|4. POST /posts - Status: draft| PostService
    PostService -->|5. Save Post & Publish Event| Kafka[Kafka Topic: media-processing]
    
    Kafka -->|6. Consume| MediaWorker[Media Transcoding Worker]
    MediaWorker -->|7. Download Raw File| RawS3
    MediaWorker -->|8. Compress/Convert WebP / Transcode HLS| MediaWorker
    MediaWorker -->|9. Upload Processed Files| ProdS3[(S3 Bucket: production-media)]
    MediaWorker -->|10. Update Post Status: active| MongoDB[(MongoDB Post DB)]
```

*   **Tính năng bổ sung**: Trực quan hóa tiến trình xử lý media cho người dùng qua WebSocket Node bằng cách gửi các frame cập nhật tiến độ (ví dụ: `MEDIA_PROCESSING_PROGRESS: 50%`).

---

## 7. Hệ thống Push Notification thực tế (FCM & APNs)

Xây dựng microservice **Notification Service** chịu trách nhiệm gửi thông báo đẩy thực tế lên thiết bị của người dùng qua Firebase Cloud Messaging (FCM) và Apple Push Notification Service (APNs).

### 7.1 Kiến trúc Notification Service

```mermaid
graph TD
    Kafka[Kafka Topic: push-notifications] -->|Consume| NotiService[Notification Service]
    NotiService -->|1. Get User Push Tokens| Mongo[(MongoDB User Tokens)]
    NotiService -->|2. Call API| FCM[Firebase Cloud Messaging API]
    NotiService -->|2. Call API| APNs[Apple Push Notification Service]
    FCM -->|Push Notification| Android[Android Devices]
    APNs -->|Push Notification| iOS[iOS Devices]
```

*   **Quản lý Token:** Cung cấp API `/api/v1/notifications/tokens` cho Client để đăng ký và cập nhật FCM/APNs token của thiết bị mỗi khi người dùng cài đặt lại app hoặc đăng nhập trên thiết bị mới.

---

## 8. Giám sát & Giới hạn Tần suất (Observability & Rate Limiting)

### 8.1 Distributed Tracing (Theo dõi phân tán) với OpenTelemetry
Tích hợp OpenTelemetry (OTel) SDK vào tất cả các microservices để theo dõi toàn bộ vòng đời của một request qua nhiều service khác nhau:

*   **Cơ chế truyền Context (Context Propagation):**
    *   *HTTP Calls:* Inject `traceparent` (W3C Trace Context) vào HTTP Headers.
    *   *Kafka Messages:* Inject trace context vào Kafka Message Headers.
    *   *gRPC Calls:* Sử dụng metadata.
*   **Visualization:** Toàn bộ dữ liệu trace được xuất ra Collector và hiển thị trực quan trên **Jaeger** hoặc **Grafana Tempo**. Từ đó có thể phát hiện nghẽn cổ chai (bottleneck) ở service nào hay truy vấn DB nào chậm.

### 8.2 Sliding Window Rate Limiting bằng Redis
Áp dụng thuật toán Sliding Window Counter sử dụng Redis Sorted Set (ZSET) tại API Gateway để ngăn chặn tấn công DDoS, Spam API:

*   **Cơ chế hoạt động:**
    1.  Mỗi request gửi đến sẽ tương ứng với một phần tử trong ZSET với value và score là timestamp hiện tại.
    2.  Dùng lệnh `ZREMRANGEBYSCORE key 0 (now - window_size)` để xoá các request nằm ngoài cửa sổ thời gian (ví dụ: ngoài 1 phút vừa qua).
    3.  Dùng `ZCARD key` để đếm số request còn lại trong cửa sổ. Nếu vượt quá giới hạn, trả về mã lỗi `429 Too Many Requests`.
    4.  Cập nhật TTL cho key bằng window size.
