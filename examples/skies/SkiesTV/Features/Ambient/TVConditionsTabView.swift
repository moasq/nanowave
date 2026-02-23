import SwiftUI

struct TVConditionsTabView: View {
    let current: CurrentWeather
    var body: some View {
        ZStack {
            conditionGradient
            TVCloudParallaxView()
            conditionContent
        }
    }
    private var conditionGradient: some View {
        LinearGradient(
            colors: current.condition.gradientColors,
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
        .ignoresSafeArea()
    }
    private var conditionContent: some View {
        VStack(spacing: AppTheme.Spacing.lg) {
            Image(systemName: current.condition.sfSymbolName)
                .font(.system(size: 120))
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(.white)
                .symbolEffect(.variableColor.iterative)
                .accessibilityLabel(current.condition.label)
            Text("\(Int(current.temperatureFahrenheit.rounded()))°")
                .font(.system(size: 120, weight: .bold, design: .rounded))
                .foregroundStyle(.white)
                .monospacedDigit()
            Text(current.condition.label)
                .font(.title)
                .foregroundStyle(.white.opacity(0.8))
            statsBadgesRow
        }
    }
    private var statsBadgesRow: some View {
        HStack(spacing: AppTheme.Spacing.xl) {
            tvStatBadge(icon: "humidity.fill", value: "\(current.humidityPercent)%", label: "Humidity")
            tvStatBadge(icon: "wind", value: "\(Int(current.windSpeedMPH.rounded())) mph", label: "Wind")
            tvStatBadge(icon: "sun.max.fill", value: "\(current.uvIndex)", label: "UV Index")
            tvStatBadge(icon: "thermometer.medium", value: "\(Int(current.feelsLikeFahrenheit.rounded()))°", label: "Feels Like")
        }
    }
    private func tvStatBadge(icon: String, value: String, label: String) -> some View {
        VStack(spacing: AppTheme.Spacing.sm) {
            Image(systemName: icon)
                .font(.title2)
                .foregroundStyle(AppTheme.Colors.primary)
                .accessibilityHidden(true)
            Text(value)
                .font(.title3)
                .bold()
                .foregroundStyle(.white)
                .monospacedDigit()
            Text(label)
                .font(.caption)
                .foregroundStyle(.white.opacity(0.7))
        }
        .accessibilityElement(children: .combine)
    }
}

#Preview {
    TVConditionsTabView(current: .sampleData)
}
