import Foundation

struct DailyForecast: Identifiable, Hashable, Codable, Sendable {
    var id: UUID
    var date: Date
    var condition: WeatherCondition
    var highFahrenheit: Double
    var lowFahrenheit: Double
    static let sampleData: [DailyForecast] = {
        let calendar = Calendar.current
        let today = calendar.startOfDay(for: Date())
        let conditions: [WeatherCondition] = [.sunny, .cloudy, .rainy, .stormy, .snowy, .windy, .sunny]
        let highs: [Double] = [75, 68, 62, 58, 45, 70, 78]
        let lows: [Double] = [58, 52, 48, 42, 30, 55, 60]
        return (0..<7).map { i in
            DailyForecast(
                id: UUID(),
                date: calendar.date(byAdding: .day, value: i, to: today) ?? today,
                condition: conditions[i],
                highFahrenheit: highs[i],
                lowFahrenheit: lows[i]
            )
        }
    }()
}
