# Tài liệu đặc tả thiết kế Dịch vụ Xác thực (Auth Service)

Tài liệu này đặc tả thiết kế chi tiết cho Dịch vụ Xác thực (Auth Service) của hệ thống mạng xã hội (social-network-system), xây dựng trên ngôn ngữ **Go**, cơ sở dữ liệu **MongoDB** và **Redis**, triển khai dưới dạng **Microservice** độc lập.

---

## 1. Tổng quan hệ thống (Overview)

Auth Service chịu trách nhiệm xử lý các nghiệp vụ liên quan đến xác thực và cấp quyền:
- Đăng ký tài khoản người dùng mới (Register).
- Đăng nhập (Login) bằng Email và Mật khẩu, cấp mã Access Token và Refresh Token.
- Làm mới Access Token hết hạn bằng Refresh Token (Token Refreshing).
- Đăng xuất (Logout) thu hồi Refresh Token.
- Cung cấp giải pháp Middleware xác thực không trạng thái (Stateless JWT verification middleware) để các microservice khác có thể tái sử dụng.

---

## 2. Kiến trúc & Phân lớp (Architecture Layering)

Hệ thống áp dụng **Clean Architecture** để đảm bảo tính độc lập, dễ kiểm thử và mở rộng:

```
+--------------------------------------------------------------+
|                     Delivery Layer (HTTP)                    |
|          Router, Request Handlers, Middleware (Gin)          |
+------------------------------+-------------------------------+
                               |
                               v
+------------------------------+-------------------------------+
|                      UseCase Layer                           |
|       Chứa Logic Nghiệp vụ (Register, Login, v.v.)           |
+------------------------------+-------------------------------+
                               |
                               v
+------------------------------+-------------------------------+
|                      Domain Layer                            |
|             Entities & Interfaces (Cốt lõi)                  |
+------------------------------+-------------------------------+
                               ^
                               |
+------------------------------+-------------------------------+
|                     Repository Layer                         |
|     Database Drivers (MongoDB for User, Redis for Tokens)    |
+--------------------------------------------------------------+
```

---

## 3. Thiết kế Cơ sở dữ liệu (Database Schema)

### 3.1 MongoDB (Lưu trữ User)
- **Database**: `social_network`
- **Collection**: `users`

```json
{
  "_id": "ObjectID",
  "username": "string (unique, required)",
  "email": "string (unique, required, format email)",
  "password_hash": "string (bcrypt hash)",
  "created_at": "ISODate",
  "updated_at": "ISODate"
}
```

- **Chỉ mục (Indexes)**:
  - Unique index trên `username`
  - Unique index trên `email`

### 3.2 Redis (Lưu trữ Refresh Token)
- **Key**: `refresh_token:<refresh_token_string>`
- **Value**: `user_id` (Dưới dạng string ObjectID)
- **TTL (Time to Live)**: 7 ngày (`604800` giây). Khi hết thời gian này, Redis sẽ tự động giải phóng bộ nhớ và Token mất hiệu lực.

---

## 4. Đặc tả API Endpoints

Cấu trúc URL API mặc định: `/api/v1/auth`

### 4.1 Đăng ký tài khoản (Register)
- **Method**: `POST`
- **Endpoint**: `/register`
- **Request Body**:
```json
{
  "username": "jane_doe",
  "email": "jane@example.com",
  "password": "SecurePassword123"
}
```
- **Response (201 Created)**:
```json
{
  "success": true,
  "message": "User registered successfully",
  "data": {
    "id": "60d5ec49f333333333333333",
    "username": "jane_doe",
    "email": "jane@example.com"
  }
}
```

### 4.2 Đăng nhập (Login)
- **Method**: `POST`
- **Endpoint**: `/login`
- **Request Body**:
```json
{
  "email": "jane@example.com",
  "password": "SecurePassword123"
}
```
- **Response (200 OK)**:
```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5...",
    "refresh_token": "a1b2c3d4-e5f6-7890-..."
  }
}
```

### 4.3 Làm mới Access Token (Refresh)
- **Method**: `POST`
- **Endpoint**: `/refresh`
- **Request Body**:
```json
{
  "refresh_token": "a1b2c3d4-e5f6-7890-..."
}
```
- **Response (200 OK)**:
```json
{
  "success": true,
  "message": "Token refreshed successfully",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5..."
  }
}
```

### 4.4 Đăng xuất (Logout)
- **Method**: `POST`
- **Endpoint**: `/logout`
- **Request Body**:
```json
{
  "refresh_token": "a1b2c3d4-e5f6-7890-..."
}
```
- **Response (200 OK)**:
```json
{
  "success": true,
  "message": "Logged out successfully",
  "data": null
}
```

### 4.5 Lấy thông tin bản thân (Me - Dùng để test Middleware)
- **Method**: `GET`
- **Endpoint**: `/me`
- **Headers**:
  - `Authorization: Bearer <access_token>`
- **Response (200 OK)**:
```json
{
  "success": true,
  "message": "Get user profile successful",
  "data": {
    "user_id": "60d5ec49f333333333333333"
  }
}
```

---

## 5. Đặc tả Token JWT
1. **Access Token**:
   - Sử dụng thuật toán ký `HS256`.
   - Payload chứa claims:
     - `sub`: User ID (ObjectID)
     - `exp`: Hạn hết hiệu lực (15 phút sau khi tạo)
     - `iat`: Thời gian tạo
2. **Refresh Token**:
   - Có thể là một chuỗi UUID ngẫu nhiên duy nhất nhằm giảm kích thước token truyền tải và tăng bảo mật khi lưu trữ trạng thái phiên làm việc trong Redis.

---

## 6. Mô hình trừu tượng hóa và Sử dụng lại (`pkg/`)
- **`pkg/jwtutil`**: Thực hiện ký và giải mã Token JWT qua khoá bí mật `JWT_SECRET`.
- **`pkg/middleware`**: Middleware chặn HTTP request, trích xuất `Authorization: Bearer <token>`, gọi `jwtutil` để giải mã và kiểm chứng. Nếu hợp lệ, gán `user_id` vào context (`c.Set("user_id", claims.Subject)`).
- **`pkg/hash`**: Lớp bao của thư viện `bcrypt`, cung cấp interface để dễ mock khi viết Unit Test.
- **`pkg/response`**: Cung cấp cấu trúc dữ liệu JSON phản hồi chuẩn `success` (true/false), `message`, `data` để đảm bảo định dạng API đồng nhất giữa các microservice.
