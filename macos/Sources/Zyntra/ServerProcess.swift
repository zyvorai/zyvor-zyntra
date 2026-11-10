// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import AppKit
import Foundation

@MainActor
final class ServerProcess: ObservableObject {
    enum Status: Equatable {
        case stopped
        case starting
        case ready
        case failed(String)
    }

    @Published private(set) var status: Status = .stopped
    @Published private(set) var port: Int = 0
    @Published private(set) var lastLogLine: String = ""

    private var process: Process?
    private var stdoutPipe: Pipe?
    private var stderrPipe: Pipe?
    private var readinessTask: Task<Void, Never>?

    var isRunning: Bool {
        switch status {
        case .starting, .ready: return true
        default: return false
        }
    }

    var isReady: Bool {
        if case .ready = status { return true }
        return false
    }

    var baseURL: URL? {
        guard port > 0 else { return nil }
        return URL(string: "http://127.0.0.1:\(port)/")
    }

    func start(using settings: AppSettings) async {
        if isRunning { return }
        status = .starting
        lastLogLine = ""

        guard let binary = Self.zyntraBinaryURL() else {
            status = .failed("Bundled zyntra binary not found. Rebuild with make mac-app.")
            return
        }
        guard let pack = settings.resolvedPackURL() else {
            status = .failed("No pack selected. Choose a pack in Settings or keep the bundled sample.")
            return
        }

        let listenPort: Int
        if settings.fixedPort > 0 {
            listenPort = settings.fixedPort
        } else {
            do {
                listenPort = try Self.freePort()
            } catch {
                status = .failed("Could not reserve a localhost port: \(error.localizedDescription)")
                return
            }
        }
        port = listenPort

        let stateDir = Self.applicationSupportURL().appendingPathComponent("state", isDirectory: true)
        try? FileManager.default.createDirectory(at: stateDir, withIntermediateDirectories: true)

        settings.saveAPIKey()

        let proc = Process()
        proc.executableURL = binary
        proc.arguments = [
            "serve",
            "-f", pack.path,
            "-addr", "127.0.0.1:\(listenPort)",
        ]
        var env = ProcessInfo.processInfo.environment
        env["ZYNTRA_API_KEY"] = settings.apiKey.isEmpty ? "dev" : settings.apiKey
        env["ZYNTRA_STATE_DIR"] = stateDir.path
        env["ZYNTRA_LISTEN"] = "127.0.0.1:\(listenPort)"
        proc.environment = env
        proc.currentDirectoryURL = pack

        let out = Pipe()
        let err = Pipe()
        proc.standardOutput = out
        proc.standardError = err
        stdoutPipe = out
        stderrPipe = err

        out.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty, let line = String(data: data, encoding: .utf8)?
                .trimmingCharacters(in: .whitespacesAndNewlines), !line.isEmpty else { return }
            Task { @MainActor in
                self?.lastLogLine = line
            }
        }
        err.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty, let line = String(data: data, encoding: .utf8)?
                .trimmingCharacters(in: .whitespacesAndNewlines), !line.isEmpty else { return }
            Task { @MainActor in
                self?.lastLogLine = line
            }
        }

        proc.terminationHandler = { [weak self] p in
            Task { @MainActor in
                guard let self else { return }
                if case .starting = self.status, p.terminationStatus != 0 {
                    self.status = .failed("Server exited (status \(p.terminationStatus)). \(self.lastLogLine)")
                } else if case .ready = self.status {
                    self.status = .stopped
                }
                self.process = nil
            }
        }

        do {
            try proc.run()
        } catch {
            status = .failed("Failed to start zyntra: \(error.localizedDescription)")
            return
        }
        process = proc

        readinessTask?.cancel()
        readinessTask = Task { [weak self] in
            guard let self else { return }
            let ok = await self.waitUntilReady(port: listenPort, timeout: 30)
            guard !Task.isCancelled else { return }
            if ok {
                self.status = .ready
            } else if self.process?.isRunning == true {
                self.status = .failed("Server started but /healthz did not become ready.")
            }
        }
    }

    func stop() {
        readinessTask?.cancel()
        readinessTask = nil
        stdoutPipe?.fileHandleForReading.readabilityHandler = nil
        stderrPipe?.fileHandleForReading.readabilityHandler = nil
        if let process, process.isRunning {
            process.terminate()
            // Give it a moment; force-kill if needed.
            DispatchQueue.global().asyncAfter(deadline: .now() + 2) {
                if process.isRunning {
                    process.interrupt()
                }
            }
        }
        process = nil
        status = .stopped
    }

    func restart(using settings: AppSettings) async {
        stop()
        // Brief pause so the port is released.
        try? await Task.sleep(nanoseconds: 300_000_000)
        await start(using: settings)
    }

    func openInBrowser() {
        guard let url = baseURL else { return }
        NSWorkspace.shared.open(url)
    }

    func url(for destination: SidebarDestination) -> URL? {
        guard let base = baseURL else { return nil }
        return URL(string: destination.hashPath, relativeTo: base)?.absoluteURL
    }

    private func waitUntilReady(port: Int, timeout: TimeInterval) async -> Bool {
        let deadline = Date().addingTimeInterval(timeout)
        guard let url = URL(string: "http://127.0.0.1:\(port)/healthz") else { return false }
        while Date() < deadline {
            if Task.isCancelled { return false }
            do {
                let (_, response) = try await URLSession.shared.data(from: url)
                if let http = response as? HTTPURLResponse, http.statusCode == 200 {
                    return true
                }
            } catch {
                // keep polling
            }
            try? await Task.sleep(nanoseconds: 200_000_000)
        }
        return false
    }

    nonisolated static func zyntraBinaryURL() -> URL? {
        let fm = FileManager.default
        if let exec = Bundle.main.executableURL {
            // zyntra-engine (not "zyntra") — avoids colliding with Zyntra on case-insensitive APFS.
            let sibling = exec.deletingLastPathComponent().appendingPathComponent("zyntra-engine")
            if fm.isExecutableFile(atPath: sibling.path) {
                return sibling
            }
        }
        if let res = Bundle.main.resourceURL?.appendingPathComponent("zyntra-engine") {
            if fm.isExecutableFile(atPath: res.path) {
                return res
            }
        }
        // Dev fallback: repo bin/ next to macos/ when running from swift build
        let cwd = URL(fileURLWithPath: fm.currentDirectoryPath)
        let candidates = [
            cwd.appendingPathComponent("bin/zyntra"),
            cwd.appendingPathComponent("../bin/zyntra"),
            cwd.deletingLastPathComponent().appendingPathComponent("bin/zyntra"),
        ]
        for c in candidates where fm.isExecutableFile(atPath: c.path) {
            return c
        }
        return nil
    }

    nonisolated static func applicationSupportURL() -> URL {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
            ?? URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Application Support")
        let dir = base.appendingPathComponent("Zyntra", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    nonisolated static func freePort() throws -> Int {
        let socket = socket(AF_INET, SOCK_STREAM, 0)
        guard socket >= 0 else { throw PortError.socket }
        defer { close(socket) }

        var addr = sockaddr_in()
        addr.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
        addr.sin_family = sa_family_t(AF_INET)
        addr.sin_port = 0
        addr.sin_addr = in_addr(s_addr: inet_addr("127.0.0.1"))

        let bindResult = withUnsafePointer(to: &addr) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                bind(socket, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
            }
        }
        guard bindResult == 0 else { throw PortError.bind }

        var length = socklen_t(MemoryLayout<sockaddr_in>.size)
        let getsock = withUnsafeMutablePointer(to: &addr) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                getsockname(socket, $0, &length)
            }
        }
        guard getsock == 0 else { throw PortError.getsockname }
        return Int(UInt16(bigEndian: addr.sin_port))
    }

    enum PortError: Error {
        case socket, bind, getsockname
    }
}
