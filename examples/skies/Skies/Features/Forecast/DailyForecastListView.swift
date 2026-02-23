import SwiftUI

struct DailyForecastListView: View {
    let daily: [DailyForecast]
    var body: some View {
        if daily.isEmpty {
            ContentUnavailableView("No Forecast", systemImage: "calendar", description: Text("Daily forecast unavailable"))
        } else {
            VStack(spacing: AppTheme.Spacing.sm) {
                ForEach(daily) { day in
                    DailyForecastRow(day: day)
                }
            }
            .padding(.horizontal, AppTheme.Spacing.md)
        }
    }
}

private struct DailyForecastRow: View {
    let day: DailyForecast
    var body: some View {
        HStack {
            Text(day.date, format: .dateTime.weekday(.abbreviated))
                .font(.callout)
                .foregroundStyle(.white)
                .frame(width: 44, alignment: .leading)
            Image(systemName: day.condition.sfSymbolName)
                .font(.body)
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(.white)
                .frame(width: 30)
                .accessibilityLabel(day.condition.label)
            Spacer()
            HStack(spacing: AppTheme.Spacing.sm) {
                Text("\(Int(day.lowFahrenheit.rounded()))°")
                    .font(.callout)
                    .foregroundStyle(.white.opacity(0.6))
                    .monospacedDigit()
                temperatureBar
                Text("\(Int(day.highFahrenheit.rounded()))°")
                    .font(.callout)
                    .bold()
                    .foregroundStyle(.white)
                    .monospacedDigit()
            }
        }
        .padding(.vertical, AppTheme.Spacing.sm)
        .padding(.horizontal, AppTheme.Spacing.md)
        .background(.ultraThinMaterial)
        .clipShape(.rect(cornerRadius: AppTheme.Style.cardCornerRadius))
        .accessibilityElement(children: .combine)
    }
    private var temperatureBar: some View {
        Capsule()
            .fill(
                LinearGradient(
                    colors: [AppTheme.Colors.secondary, AppTheme.Colors.primary],
                    startPoint: .leading,
                    endPoint: .trailing
                )
            )
            .frame(width: 60, height: 4)
    }
}

#Preview {
    ZStack {
        AppTheme.Colors.background.ignoresSafeArea()
        DailyForecastListView(daily: DailyForecast.sampleData)
    }
}
