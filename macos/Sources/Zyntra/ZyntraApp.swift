// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import SwiftUI

@main
struct ZyntraApp: App {
    @StateObject private var server = ServerProcess()
    @StateObject private var settings = AppSettings()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(server)
                .environmentObject(settings)
                .frame(minWidth: 1100, minHeight: 700)
                .onAppear {
                    AppTerminationObserver.shared.install(server: server)
                }
        }
        .defaultSize(width: 1280, height: 820)
        .commands {
            CommandGroup(replacing: .newItem) {}
            CommandMenu("Server") {
                Button(server.isRunning ? "Restart Server" : "Start Server") {
                    Task { await server.restart(using: settings) }
                }
                .keyboardShortcut("r", modifiers: [.command, .shift])

                Button("Stop Server") {
                    server.stop()
                }
                .disabled(!server.isRunning)

                Divider()

                Button("Open Console in Browser") {
                    server.openInBrowser()
                }
                .disabled(!server.isReady)
                .keyboardShortcut("o", modifiers: [.command, .shift])
            }
            CommandGroup(after: .help) {
                Link("Zyntra docs", destination: URL(string: "https://zyvorai.github.io/zyvor-zyntra/")!)
                Link("zyvor.dev", destination: URL(string: "https://zyvor.dev/zyntra")!)
            }
        }

        Settings {
            SettingsView()
                .environmentObject(server)
                .environmentObject(settings)
        }
    }
}
