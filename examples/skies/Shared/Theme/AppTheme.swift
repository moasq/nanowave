import SwiftUI

enum AppTheme {
    enum Colors {
        static let primary = Color(hex: "F4A825")
        static let secondary = Color(hex: "4A90D9")
        static let accent = Color(hex: "7B5EA7")
        static let background = Color(hex: "0D1B2A")
        static let surface = Color(hex: "1A2E42")
    }
    enum Spacing {
        static let xs: CGFloat = 4
        static let sm: CGFloat = 8
        static let md: CGFloat = 16
        static let lg: CGFloat = 24
        static let xl: CGFloat = 40
    }
    enum Style {
        static let cornerRadius: CGFloat = 20
        static let cardCornerRadius: CGFloat = 16
    }
    static func gradientFor(_ condition: WeatherCondition) -> LinearGradient {
        LinearGradient(
            colors: condition.gradientColors,
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
    }
}

extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: .init(charactersIn: "#"))
        let scanner = Scanner(string: hex)
        var rgbValue: UInt64 = 0
        scanner.scanHexInt64(&rgbValue)
        self.init(
            red: Double((rgbValue & 0xFF0000) >> 16) / 255.0,
            green: Double((rgbValue & 0x00FF00) >> 8) / 255.0,
            blue: Double(rgbValue & 0x0000FF) / 255.0
        )
    }
}
