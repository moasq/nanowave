import SwiftUI

struct TVDailyTabView: View {
    let daily: [DailyForecast]
    var body: some View {
        VStack(alignment: .leading, spacing: AppTheme.Spacing.lg) {
            Text("7-Day Forecast")
                .font(.title2)
                .bold()
                .foregroundStyle(.white)
                .padding(.horizontal, 80)
            if daily.isEmpty {
                ContentUnavailableView("No Forecast", systemImage: "calendar", description: Text("Daily forecast unavailable"))
            } else {
                HStack(spacing: AppTheme.Spacing.xl) {
                    ForEach(daily) { day in
                        tvDailyCard(day: day)
                    }
                }
                .padding(.horizontal, 80)
            }
        }
    }
    private func tvDailyCard(day: DailyForecast) -> some View {
        VStack(spacing: AppTheme.Spacing.md) {
            Text(day.date, format: .dateTime.weekday(.abbreviated))
                .font(.callout)
                .foregroundStyle(.white.opacity(0.8))
            Image(systemName: day.condition.sfSymbolName)
                .font(.title)
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(.white)
                .accessibilityLabel(day.condition.label)
            Text("\(Int(day.highFahrenheit.rounded()))°")
                .font(.title3)
                .bold()
                .foregroundStyle(.white)
                .monospacedDigit()
            Text("\(Int(day.lowFahrenheit.rounded()))°")
                .font(.callout)
                .foregroundStyle(.white.opacity(0.6))
                .monospacedDigit()
        }
        .frame(maxWidth: .infinity)
        .padding(AppTheme.Spacing.lg)
        .background(.ultraThinMaterial)
        .clipShape(.rect(cornerRadius: AppTheme.Style.cardCornerRadius))
        .focusable()
    }
}

#Preview {
    ZStack {
        Color(hex: "0D1B2A").ignoresSafeArea()
        TVDailyTabView(daily: DailyForecast.sampleData)
    }
}
