import SwiftUI

struct CurrentConditionsHeroView: View {
    let current: CurrentWeather
    var body: some View {
        VStack(spacing: AppTheme.Spacing.md) {
            weatherIcon
            temperatureDisplay
            statsRow
        }
        .padding(AppTheme.Spacing.lg)
        .glassCard()
    }
    private var weatherIcon: some View {
        Image(systemName: current.condition.sfSymbolName)
            .font(.system(size: 64))
            .symbolRenderingMode(.hierarchical)
            .foregroundStyle(.white)
            .symbolEffect(.variableColor.iterative)
            .accessibilityLabel(current.condition.label)
    }
    private var temperatureDisplay: some View {
        VStack(spacing: AppTheme.Spacing.xs) {
            Text("\(Int(current.temperatureFahrenheit.rounded()))°")
                .font(.system(.largeTitle, design: .rounded, weight: .bold))
                .foregroundStyle(.white)
            Text("Feels like \(Int(current.feelsLikeFahrenheit.rounded()))°")
                .font(.subheadline)
                .foregroundStyle(.white.opacity(0.8))
            Text(current.locationName)
                .font(.headline)
                .foregroundStyle(.white.opacity(0.9))
        }
    }
    private var statsRow: some View {
        HStack(spacing: AppTheme.Spacing.lg) {
            statBadge(icon: "humidity.fill", value: "\(current.humidityPercent)%", label: "Humidity")
            statBadge(icon: "wind", value: "\(Int(current.windSpeedMPH.rounded())) mph", label: "Wind")
            statBadge(icon: "sun.max.fill", value: "\(current.uvIndex)", label: "UV Index")
        }
    }
    private func statBadge(icon: String, value: String, label: String) -> some View {
        VStack(spacing: AppTheme.Spacing.xs) {
            Image(systemName: icon)
                .font(.caption)
                .foregroundStyle(AppTheme.Colors.primary)
                .accessibilityHidden(true)
            Text(value)
                .font(.callout)
                .bold()
                .foregroundStyle(.white)
                .monospacedDigit()
            Text(label)
                .font(.caption2)
                .foregroundStyle(.white.opacity(0.7))
        }
        .accessibilityElement(children: .combine)
    }
}

#Preview {
    ZStack {
        ConditionBackgroundView(condition: .sunny)
        CurrentConditionsHeroView(current: .sampleData)
    }
}
