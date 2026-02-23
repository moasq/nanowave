import Foundation

struct HourlyForecast: Identifiable, Hashable, Codable, Sendable {
    var id: UUID
    var hour: Date
    var condition: WeatherCondition
    var temperatureFahrenheit: Double
    static let sampleData: [HourlyForecast] = {
        let calendar = Calendar.current
        let now = Date()
        let conditions: [WeatherCondition] = [.sunny, .sunny, .cloudy, .cloudy, .rainy, .rainy, .stormy, .cloudy, .cloudy, .sunny, .sunny, .sunny, .windy, .windy, .cloudy, .cloudy, .rainy, .snowy, .snowy, .cloudy, .sunny, .sunny, .sunny, .sunny]
        let temps: [Double] = [72, 73, 71, 69, 65, 63, 60, 59, 58, 60, 64, 68, 70, 72, 71, 69, 66, 62, 58, 57, 59, 63, 67, 70]
        return (0..<24).map { i in
            HourlyForecast(
                id: UUID(),
                hour: calendar.date(byAdding: .hour, value: i, to: now) ?? now,
                condition: conditions[i],
                temperatureFahrenheit: temps[i]
            )
        }
    }()
}
