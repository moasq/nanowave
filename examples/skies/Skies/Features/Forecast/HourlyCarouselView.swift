import SwiftUI

struct HourlyCarouselView: View {
    let hourly: [HourlyForecast]
    var body: some View {
        if hourly.isEmpty {
            ContentUnavailableView("No Hourly Data", systemImage: "clock", description: Text("Hourly forecast unavailable"))
        } else {
            ScrollView(.horizontal, showsIndicators: false) {
                LazyHStack(spacing: AppTheme.Spacing.sm) {
                    ForEach(hourly) { slot in
                        HourlyCardView(slot: slot)
                    }
                }
                .padding(.horizontal, AppTheme.Spacing.md)
            }
        }
    }
}

private struct HourlyCardView: View {
    let slot: HourlyForecast
    var body: some View {
        VStack(spacing: AppTheme.Spacing.sm) {
            Text(slot.hour, format: .dateTime.hour())
                .font(.caption)
                .foregroundStyle(.white.opacity(0.8))
            Image(systemName: slot.condition.sfSymbolName)
                .font(.title3)
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(.white)
                .accessibilityLabel(slot.condition.label)
            Text("\(Int(slot.temperatureFahrenheit.rounded()))°")
                .font(.callout)
                .bold()
                .foregroundStyle(.white)
                .monospacedDigit()
        }
        .frame(width: 60)
        .padding(.vertical, AppTheme.Spacing.sm)
        .glassCard()
    }
}

#Preview {
    ZStack {
        AppTheme.Colors.background.ignoresSafeArea()
        HourlyCarouselView(hourly: HourlyForecast.sampleData)
    }
}
