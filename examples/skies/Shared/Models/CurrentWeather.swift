import Foundation

struct CurrentWeather: Identifiable, Hashable, Codable, Sendable {
    var id: UUID
    var condition: WeatherCondition
    var temperatureFahrenheit: Double
    var feelsLikeFahrenheit: Double
    var humidityPercent: Int
    var windSpeedMPH: Double
    var uvIndex: Int
    var locationName: String
    static let sampleData = CurrentWeather(
        id: UUID(),
        condition: .sunny,
        temperatureFahrenheit: 72,
        feelsLikeFahrenheit: 74,
        humidityPercent: 45,
        windSpeedMPH: 8.5,
        uvIndex: 6,
        locationName: "San Francisco"
    )
}
