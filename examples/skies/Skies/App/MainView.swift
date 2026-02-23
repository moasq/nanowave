import SwiftUI

struct MainView: View {
    @EnvironmentObject var store: WeatherStore
    @Environment(\.horizontalSizeClass) private var sizeClass
    var body: some View {
        if sizeClass == .regular {
            iPadLayout
        } else {
            iPhoneLayout
        }
    }
    private var iPhoneLayout: some View {
        ZStack {
            ConditionBackgroundView(condition: store.snapshot.current.condition)
            ScrollView {
                VStack(spacing: AppTheme.Spacing.lg) {
                    CurrentConditionsHeroView(current: store.snapshot.current)
                    sectionHeader("Hourly Forecast")
                    HourlyCarouselView(hourly: store.snapshot.hourly)
                    sectionHeader("7-Day Forecast")
                    DailyForecastListView(daily: store.snapshot.daily)
                }
                .padding(.vertical, AppTheme.Spacing.lg)
            }
        }
    }
    private var iPadLayout: some View {
        NavigationSplitView {
            ZStack {
                AppTheme.Colors.background.ignoresSafeArea()
                VStack(alignment: .leading, spacing: AppTheme.Spacing.md) {
                    Text("7-Day Forecast")
                        .font(.title2)
                        .bold()
                        .foregroundStyle(.white)
                        .padding(.horizontal, AppTheme.Spacing.md)
                    DailyForecastListView(daily: store.snapshot.daily)
                }
                .padding(.vertical, AppTheme.Spacing.md)
            }
            .navigationTitle("Skies")
        } detail: {
            ZStack {
                ConditionBackgroundView(condition: store.snapshot.current.condition)
                ScrollView {
                    VStack(spacing: AppTheme.Spacing.lg) {
                        CurrentConditionsHeroView(current: store.snapshot.current)
                        sectionHeader("Hourly Forecast")
                        HourlyCarouselView(hourly: store.snapshot.hourly)
                    }
                    .padding(.vertical, AppTheme.Spacing.lg)
                }
            }
        }
    }
    private func sectionHeader(_ title: String) -> some View {
        Text(title)
            .font(.title3)
            .bold()
            .foregroundStyle(.white)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, AppTheme.Spacing.md)
    }
}

#Preview {
    MainView()
        .environmentObject(WeatherStore.shared)
}
