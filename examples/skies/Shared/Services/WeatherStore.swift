import SwiftUI

@MainActor
final class WeatherStore: ObservableObject, @unchecked Sendable {
    @Published var snapshot: WeatherSnapshot
    static let shared = WeatherStore()
    private init() {
        snapshot = Self.generateSnapshot()
    }
    func refreshWeather() {
        snapshot = Self.generateSnapshot()
    }
    func simulateConditionChange() {
        let conditions = WeatherCondition.allCases
        let newCondition = conditions.randomElement() ?? .sunny
        var current = snapshot.current
        current.condition = newCondition
        current.temperatureFahrenheit = Double.random(in: 40...95)
        current.feelsLikeFahrenheit = current.temperatureFahrenheit + Double.random(in: -3...5)
        current.humidityPercent = Int.random(in: 20...90)
        current.windSpeedMPH = Double.random(in: 0...30)
        current.uvIndex = Int.random(in: 0...11)
        snapshot.current = current
    }
    private static func generateSnapshot() -> WeatherSnapshot {
        let conditions = WeatherCondition.allCases
        let calendar = Calendar.current
        let now = Date()
        let current = CurrentWeather(
            id: UUID(),
            condition: conditions.randomElement() ?? .sunny,
            temperatureFahrenheit: Double.random(in: 45...90),
            feelsLikeFahrenheit: Double.random(in: 42...95),
            humidityPercent: Int.random(in: 20...85),
            windSpeedMPH: Double.random(in: 2...25),
            uvIndex: Int.random(in: 0...11),
            locationName: "San Francisco"
        )
        let hourly = (0..<24).map { i in
            HourlyForecast(
                id: UUID(),
                hour: calendar.date(byAdding: .hour, value: i, to: now) ?? now,
                condition: conditions.randomElement() ?? .cloudy,
                temperatureFahrenheit: Double.random(in: 45...90)
            )
        }
        let today = calendar.startOfDay(for: now)
        let daily = (0..<7).map { i in
            let high = Double.random(in: 55...95)
            return DailyForecast(
                id: UUID(),
                date: calendar.date(byAdding: .day, value: i, to: today) ?? today,
                condition: conditions.randomElement() ?? .sunny,
                highFahrenheit: high,
                lowFahrenheit: high - Double.random(in: 10...25)
            )
        }
        return WeatherSnapshot(id: UUID(), current: current, hourly: hourly, daily: daily)
    }
}
