import Foundation

/// 后端成功封装：{"data": T}
struct DataEnvelope<T: Decodable>: Decodable {
    let data: T
}

/// 后端失败封装：{"error": {"code","message"}}
struct ErrorEnvelope: Decodable {
    let error: APIError
}
