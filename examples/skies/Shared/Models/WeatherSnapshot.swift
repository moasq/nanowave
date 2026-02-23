import Foundation

struct WeatherSnapshot: Identifiable, Hashable, Codable, Sendable {
    var id: UUID
    var current: CurrentWeather
    var hourly: [HourlyForecast]
    var daily: [DailyForecast]
    static let sampleData = WeatherSnapshot(
        id: UUID(),
        current: CurrentWeather.sampleData,
        hourly: HourlyForecast.sampleData,
        daily: DailyForecast.sampleData
    )
}
