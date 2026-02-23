import SwiftUI

@main
struct SkiesApp: App {
    @StateObject private var store = WeatherStore.shared
    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(store)
        }
    }
}
