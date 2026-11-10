// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import AppKit
import SwiftUI

/// Ensures the child `zyntra` process is torn down when the app quits.
final class AppTerminationObserver {
    static let shared = AppTerminationObserver()
    weak var server: ServerProcess?

    private var token: NSObjectProtocol?

    func install(server: ServerProcess) {
        self.server = server
        guard token == nil else { return }
        token = NotificationCenter.default.addObserver(
            forName: NSApplication.willTerminateNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            Task { @MainActor in
                self?.server?.stop()
            }
        }
    }
}
