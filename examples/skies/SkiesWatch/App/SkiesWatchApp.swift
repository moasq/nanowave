import SwiftUI

@main
struct SkiesWatchApp: App {
    @StateObject private var store = WeatherStore.shared
    var body: some Scene {
        WindowGroup {
            WatchRootView()
                .environmentObject(store)
        }
    }
}
