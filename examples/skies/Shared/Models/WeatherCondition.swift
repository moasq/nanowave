import SwiftUI

enum WeatherCondition: String, Codable, CaseIterable, Sendable {
    case sunny, rainy, stormy, snowy, cloudy, windy
    var gradientColors: [Color] {
        switch self {
        case .sunny: [Color(hex: "F4A825"), Color(hex: "FF6B35"), Color(hex: "FFD700")]
        case .rainy: [Color(hex: "4A90D9"), Color(hex: "2C5F8A"), Color(hex: "1A3A5C")]
        case .stormy: [Color(hex: "7B5EA7"), Color(hex: "3D2B5A"), Color(hex: "1A0E2E")]
        case .snowy: [Color(hex: "B8C6DB"), Color(hex: "8FA4C4"), Color(hex: "F0F4F8")]
        case .cloudy: [Color(hex: "6B7B8D"), Color(hex: "4A5568"), Color(hex: "2D3748")]
        case .windy: [Color(hex: "4A90D9"), Color(hex: "6DB3F2"), Color(hex: "A3D9FF")]
        }
    }
    var sfSymbolName: String {
        switch self {
        case .sunny: "sun.max.fill"
        case .rainy: "cloud.rain.fill"
        case .stormy: "cloud.bolt.rain.fill"
        case .snowy: "cloud.snow.fill"
        case .cloudy: "cloud.fill"
        case .windy: "wind"
        }
    }
    var animatedSymbolName: String {
        sfSymbolName
    }
    var label: String {
        switch self {
        case .sunny: "Sunny"
        case .rainy: "Rainy"
        case .stormy: "Stormy"
        case .snowy: "Snowy"
        case .cloudy: "Cloudy"
        case .windy: "Windy"
        }
    }
}
