import SwiftUI

@Observable
@MainActor
final class DashboardViewModel {
    var current: CurrentWeather
    var animatedCondition: WeatherCondition
    private let store: WeatherStore
    init(store: WeatherStore) {
        self.store = store
        self.current = store.snapshot.current
        self.animatedCondition = store.snapshot.current.condition
    }
    func refresh() {
        current = store.snapshot.current
        animatedCondition = current.condition
    }
    func formattedTemperature(_ f: Double) -> String {
        "\(Int(f.rounded()))°"
    }
    func formattedWind(_ mph: Double) -> String {
        "\(Int(mph.rounded())) mph"
    }
    var conditionGradient: LinearGradient {
        AppTheme.gradientFor(animatedCondition)
    }
}
