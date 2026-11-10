// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import AppKit
import SwiftUI

struct SettingsView: View {
    @EnvironmentObject private var server: ServerProcess
    @EnvironmentObject private var settings: AppSettings

    var body: some View {
        Form {
            Section("Pack") {
                Toggle("Use bundled sample pack when no path is set", isOn: $settings.preferBundledSample)
                HStack {
                    TextField("Pack directory", text: $settings.packPath)
                    Button("Choose…") { pickPack() }
                }
                Text("Point at a pack folder (kpis.yaml / pack.yaml), same as `zyntra serve -f`.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section("Authentication") {
                SecureField("ZYNTRA_API_KEY", text: $settings.apiKey)
                Text("Stored in the Keychain. Default for the sample pack is `dev` (sign in as admin / Admin@321).")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Button("Save API key") {
                    settings.saveAPIKey()
                }
            }

            Section("Listen") {
                TextField("Fixed port (0 = automatic)", value: $settings.fixedPort, format: .number)
                Text("The app binds to 127.0.0.1 only.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section("Server") {
                LabeledContent("Status") {
                    Text(statusText)
                }
                if let url = server.baseURL {
                    LabeledContent("URL") {
                        Text(url.absoluteString)
                            .textSelection(.enabled)
                    }
                }
                HStack {
                    Button("Apply & Restart") {
                        settings.saveAPIKey()
                        Task { await server.restart(using: settings) }
                    }
                    .keyboardShortcut(.defaultAction)
                    Button("Open in Browser") {
                        server.openInBrowser()
                    }
                    .disabled(!server.isReady)
                }
            }
        }
        .formStyle(.grouped)
        .padding()
        .frame(width: 520, height: 420)
    }

    private var statusText: String {
        switch server.status {
        case .ready: return "Ready on port \(server.port)"
        case .starting: return "Starting…"
        case .stopped: return "Stopped"
        case .failed(let m): return m
        }
    }

    private func pickPack() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = false
        panel.message = "Choose a Zyntra pack directory"
        panel.directoryURL = ServerProcess.applicationSupportURL()
        if panel.runModal() == .OK, let url = panel.url {
            settings.packPath = url.path
            settings.preferBundledSample = false
        }
    }
}
