import SwiftUI

struct TVHourlyTabView: View {
    let hourly: [HourlyForecast]
    var body: some View {
        VStack(alignment: .leading, spacing: AppTheme.Spacing.lg) {
            Text("Hourly Forecast")
                .font(.title2)
                .bold()
                .foregroundStyle(.white)
                .padding(.horizontal, 80)
            if hourly.isEmpty {
                ContentUnavailableView("No Hourly Data", systemImage: "clock", description: Text("Hourly forecast unavailable"))
            } else {
                ScrollView(.horizontal, showsIndicators: false) {
                    LazyHStack(spacing: AppTheme.Spacing.lg) {
                        ForEach(hourly) { slot in
                            tvHourlyCard(slot: slot)
                        }
                    }
                    .padding(.horizontal, 80)
                }
            }
        }
    }
    private func tvHourlyCard(slot: HourlyForecast) -> some View {
        VStack(spacing: AppTheme.Spacing.md) {
            Text(slot.hour, format: .dateTime.hour())
                .font(.callout)
                .foregroundStyle(.white.opacity(0.8))
            Image(systemName: slot.condition.sfSymbolName)
                .font(.title)
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(.white)
                .accessibilityLabel(slot.condition.label)
            Text("\(Int(slot.temperatureFahrenheit.rounded()))°")
                .font(.title3)
                .bold()
                .foregroundStyle(.white)
                .monospacedDigit()
        }
        .padding(AppTheme.Spacing.lg)
        .background(.ultraThinMaterial)
        .clipShape(.rect(cornerRadius: AppTheme.Style.cardCornerRadius))
        .focusable()
    }
}

#Preview {
    ZStack {
        Color(hex: "0D1B2A").ignoresSafeArea()
        TVHourlyTabView(hourly: HourlyForecast.sampleData)
    }
}
