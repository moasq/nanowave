import SwiftUI

struct WatchMainView: View {
    let snapshot: WeatherSnapshot
    var body: some View {
        ZStack {
            conditionBackground
            VStack(spacing: 8) {
                weatherIcon
                temperatureText
                WatchMiniCarouselView(hourly: Array(snapshot.hourly.prefix(3)))
            }
            .padding(.horizontal, 4)
        }
    }
    private var conditionBackground: some View {
        LinearGradient(
            colors: snapshot.current.condition.gradientColors,
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
        .ignoresSafeArea()
    }
    private var weatherIcon: some View {
        Image(systemName: snapshot.current.condition.sfSymbolName)
            .font(.title2)
            .symbolRenderingMode(.hierarchical)
            .foregroundStyle(.white)
            .symbolEffect(.variableColor.iterative)
            .accessibilityLabel(snapshot.current.condition.label)
    }
    private var temperatureText: some View {
        Text("\(Int(snapshot.current.temperatureFahrenheit.rounded()))°")
            .font(.system(.largeTitle, design: .rounded, weight: .bold))
            .foregroundStyle(.white)
            .monospacedDigit()
    }
}

#Preview {
    WatchMainView(snapshot: .sampleData)
}
