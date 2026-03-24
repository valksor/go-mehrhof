//! Server Manager
//!
//! Manages the lifecycle of the kvelmo Go backend:
//! - Spawns the server process (sidecar on macOS/Linux, WSL on Windows)
//! - Detects the port from stdout
//! - Navigates the webview to the running server
//! - Handles graceful shutdown

use regex::Regex;
use std::sync::Arc;
use std::time::Duration;
use tauri::AppHandle;
#[cfg(not(dev))]
use tauri::{Emitter, Manager, Url};
use tauri_plugin_shell::process::CommandEvent;
use tauri_plugin_shell::ShellExt;
use tokio::sync::Mutex;
use tokio::time::timeout;

/// Global server state
static SERVER_PROCESS: std::sync::OnceLock<Arc<Mutex<Option<ServerProcess>>>> =
    std::sync::OnceLock::new();

/// Holds the running server process
struct ServerProcess {
    #[allow(dead_code)] // Used in production builds via navigate_to_server
    port: u16,
    // The child process is managed by Tauri's shell plugin
    // We just need to track that we started it
}

/// Start the kvelmo server and navigate to it (production only)
pub async fn start_server(app: &AppHandle) -> Result<(), String> {
    // Get or create the server process holder
    let server = SERVER_PROCESS.get_or_init(|| Arc::new(Mutex::new(None)));
    let mut guard = server.lock().await;

    // If already running, just navigate (production only)
    if let Some(ref _proc) = *guard {
        #[cfg(not(dev))]
        navigate_to_server(app, _proc.port)?;
        return Ok(());
    }

    // Start the server process
    let port = spawn_server(app).await?;

    // Store the process info
    *guard = Some(ServerProcess { port });

    // In production, navigate to the server
    // In dev mode, Vite proxies to the server, so no navigation needed
    #[cfg(not(dev))]
    navigate_to_server(app, port)?;

    #[cfg(dev)]
    tracing::info!(
        "Dev mode: Server running at http://localhost:{}, Vite will proxy",
        port
    );

    Ok(())
}

/// Spawn the server process and return the detected port
async fn spawn_server(app: &AppHandle) -> Result<u16, String> {
    #[cfg(target_os = "windows")]
    {
        spawn_server_wsl(app).await
    }

    #[cfg(not(target_os = "windows"))]
    {
        spawn_server_sidecar(app).await
    }
}

/// Spawn server using Tauri sidecar (macOS/Linux)
#[cfg(not(target_os = "windows"))]
async fn spawn_server_sidecar(app: &AppHandle) -> Result<u16, String> {
    let shell = app.shell();

    // In dev mode, use fixed port 6337 for Vite proxy compatibility
    // In production, use random port
    #[cfg(dev)]
    let port_arg = "6337";
    #[cfg(not(dev))]
    let port_arg = "0";

    println!("[kvelmo-desktop] Starting sidecar with port {}", port_arg);

    let sidecar = shell
        .sidecar("kvelmo")
        .map_err(|e| {
            let msg = format!("Failed to create sidecar: {}", e);
            println!("[kvelmo-desktop] {}", msg);
            msg
        })?
        .args(["serve", "--port", port_arg]);

    let (mut rx, _child) = sidecar.spawn().map_err(|e| {
        let msg = format!("Failed to spawn sidecar: {}", e);
        println!("[kvelmo-desktop] {}", msg);
        msg
    })?;

    println!("[kvelmo-desktop] Sidecar spawned, waiting for port...");

    // Wait for port detection with 30 second timeout
    wait_for_port(&mut rx).await
}

/// Spawn server via WSL (Windows)
#[cfg(target_os = "windows")]
async fn spawn_server_wsl(app: &AppHandle) -> Result<u16, String> {
    use crate::wsl;

    // Check WSL is available
    if !wsl::is_wsl_installed() {
        return Err(
            "WSL is not installed. Please install WSL from https://aka.ms/wslinstall".to_string(),
        );
    }

    // Deploy kvelmo binary if needed
    wsl::ensure_kvelmo_deployed(app).await?;

    // Spawn via WSL
    let shell = app.shell();

    // Use Command instead of sidecar for WSL
    let (mut rx, _child) = shell
        .command("wsl")
        .args(["~/.local/bin/kvelmo", "serve", "--port", "0"])
        .spawn()
        .map_err(|e| format!("Failed to spawn WSL process: {}", e))?;

    // Wait for port detection
    wait_for_port(&mut rx).await
}

/// Extract port from server output line
/// e.g., "Listening on localhost:8080" -> Some(8080)
pub(crate) fn extract_port(line: &str) -> Option<u16> {
    let regex = Regex::new(r"localhost:(\d+)").unwrap();
    regex
        .captures(line)
        .and_then(|c| c.get(1))
        .and_then(|m| m.as_str().parse().ok())
}

/// Wait for the server to output its port
async fn wait_for_port(rx: &mut tokio::sync::mpsc::Receiver<CommandEvent>) -> Result<u16, String> {
    let port_future = async {
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(line_bytes) => {
                    let line = String::from_utf8_lossy(&line_bytes);
                    println!("[kvelmo-desktop] stdout: {}", line);

                    if let Some(port) = extract_port(&line) {
                        println!("[kvelmo-desktop] Detected port: {}", port);
                        return Ok(port);
                    }
                }
                CommandEvent::Stderr(line_bytes) => {
                    let line = String::from_utf8_lossy(&line_bytes);
                    println!("[kvelmo-desktop] stderr: {}", line);
                }
                CommandEvent::Error(e) => {
                    let msg = format!("Server process error: {}", e);
                    println!("[kvelmo-desktop] {}", msg);
                    return Err(msg);
                }
                CommandEvent::Terminated(status) => {
                    let msg = format!("Server process terminated unexpectedly: {:?}", status);
                    println!("[kvelmo-desktop] {}", msg);
                    return Err(msg);
                }
                _ => {}
            }
        }
        Err("Server process closed before port was detected".to_string())
    };

    timeout(Duration::from_secs(30), port_future)
        .await
        .map_err(|_| "Server startup timed out after 30 seconds".to_string())?
}

/// Navigate the main window to the server URL
#[cfg(not(dev))]
fn navigate_to_server(app: &AppHandle, port: u16) -> Result<(), String> {
    let window = app
        .get_webview_window("main")
        .ok_or("Main window not found")?;

    // Navigate to the server with port in query string for WebSocket connection
    let url_str = format!("http://localhost:{}?port={}", port, port);
    let url = Url::parse(&url_str).map_err(|e| format!("Invalid URL: {}", e))?;

    // Use Tauri's navigate API instead of eval
    window
        .navigate(url)
        .map_err(|e| format!("Failed to navigate: {}", e))?;

    // Emit event for UI to know server is ready
    let _ = app.emit("server-ready", port);

    tracing::info!("Server running at http://localhost:{}", port);

    Ok(())
}

/// Stop the server (called on app exit)
pub async fn stop_server() {
    if let Some(server) = SERVER_PROCESS.get() {
        let mut guard = server.lock().await;
        if guard.is_some() {
            // The child process will be killed when dropped by Tauri
            *guard = None;
            tracing::info!("Server stopped");
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_extract_port_basic() {
        assert_eq!(extract_port("localhost:6337"), Some(6337));
    }

    #[test]
    fn test_extract_port_in_sentence() {
        assert_eq!(extract_port("Listening on localhost:8080"), Some(8080));
        assert_eq!(
            extract_port("Server started at localhost:52341/api"),
            Some(52341)
        );
    }

    #[test]
    fn test_extract_port_no_match() {
        assert_eq!(extract_port("Loading config..."), None);
        assert_eq!(extract_port("127.0.0.1:8080"), None); // Different format
        assert_eq!(extract_port("localhost"), None);
        assert_eq!(extract_port("localhost:abc"), None);
    }

    #[test]
    fn test_extract_port_first_match() {
        // Should extract first port found
        assert_eq!(extract_port("localhost:3000 localhost:4000"), Some(3000));
    }

    #[test]
    fn test_extract_port_boundary_min() {
        assert_eq!(extract_port("localhost:0"), Some(0));
    }

    #[test]
    fn test_extract_port_boundary_max() {
        assert_eq!(extract_port("localhost:65535"), Some(65535));
    }

    #[test]
    fn test_extract_port_overflow() {
        // 99999 exceeds u16::MAX (65535)
        assert_eq!(extract_port("localhost:99999"), None);
    }

    #[test]
    fn test_extract_port_way_over_u16() {
        assert_eq!(extract_port("localhost:100000"), None);
    }

    #[test]
    fn test_extract_port_empty_string() {
        assert_eq!(extract_port(""), None);
    }

    #[test]
    fn test_extract_port_colon_no_number() {
        assert_eq!(extract_port("localhost:"), None);
    }

    #[test]
    fn test_extract_port_multiple_colons() {
        assert_eq!(extract_port("12:34:56 localhost:8080"), Some(8080));
    }

    #[test]
    fn test_extract_port_unicode_surrounding() {
        assert_eq!(extract_port("サーバー localhost:9090 開始"), Some(9090));
    }

    #[test]
    fn test_extract_port_only_whitespace() {
        assert_eq!(extract_port("   "), None);
    }

    #[test]
    fn test_extract_port_port_one() {
        assert_eq!(extract_port("localhost:1"), Some(1));
    }
}
