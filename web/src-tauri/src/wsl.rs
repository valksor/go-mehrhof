//! WSL Integration (Windows only)
//!
//! Handles deployment and execution of kvelmo inside WSL on Windows.

use std::process::Command;
use tauri::AppHandle;
use tauri::Manager;

/// Check if WSL is installed and available
pub fn is_wsl_installed() -> bool {
    Command::new("wsl")
        .arg("--version")
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false)
}

/// Check if kvelmo is installed in WSL
pub fn is_kvelmo_installed() -> bool {
    // Check both PATH and our deployment location
    let path_check = Command::new("wsl")
        .args(["which", "kvelmo"])
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false);

    if path_check {
        return true;
    }

    // Check our deployment location
    Command::new("wsl")
        .args(["test", "-x", "~/.local/bin/kvelmo"])
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false)
}

/// Get the installed version of kvelmo in WSL
pub fn get_installed_version() -> Option<String> {
    let output = Command::new("wsl")
        .args(["~/.local/bin/kvelmo", "--version"])
        .output()
        .ok()?;

    if !output.status.success() {
        // Try PATH version
        let output = Command::new("wsl")
            .args(["kvelmo", "--version"])
            .output()
            .ok()?;

        if !output.status.success() {
            return None;
        }

        return Some(String::from_utf8_lossy(&output.stdout).trim().to_string());
    }

    Some(String::from_utf8_lossy(&output.stdout).trim().to_string())
}

/// Ensure kvelmo is deployed to WSL with correct version
pub async fn ensure_kvelmo_deployed(app: &AppHandle) -> Result<(), String> {
    let expected_version = crate::VERSION;

    // Check if already installed with correct version
    if let Some(installed) = get_installed_version() {
        if installed.contains(expected_version) {
            tracing::info!("kvelmo {} already installed in WSL", installed);
            return Ok(());
        }
        tracing::info!(
            "kvelmo version mismatch: installed={}, expected={}",
            installed,
            expected_version
        );
    }

    // Deploy the bundled binary
    deploy_binary(app).await
}

/// Deploy the bundled Linux binary to WSL
async fn deploy_binary(app: &AppHandle) -> Result<(), String> {
    tracing::info!("Deploying kvelmo to WSL...");

    // Get the resource path for the WSL binary
    let resource_path = app
        .path()
        .resource_dir()
        .map_err(|e| format!("Failed to get resource dir: {}", e))?
        .join("resources");

    // Determine which binary to use based on architecture
    let binary_name = if cfg!(target_arch = "x86_64") {
        "kvelmo-wsl-x64"
    } else if cfg!(target_arch = "aarch64") {
        "kvelmo-wsl-arm64"
    } else {
        return Err("Unsupported architecture".to_string());
    };

    let binary_path = resource_path.join(binary_name);

    if !binary_path.exists() {
        return Err(format!("WSL binary not found at {}", binary_path.display()));
    }

    // Convert Windows path to WSL path
    let wsl_source_path = windows_to_wsl_path(&binary_path.to_string_lossy());

    // Create ~/.local/bin in WSL
    let mkdir_output = Command::new("wsl")
        .args(["mkdir", "-p", "~/.local/bin"])
        .output()
        .map_err(|e| format!("Failed to create directory in WSL: {}", e))?;

    if !mkdir_output.status.success() {
        return Err(format!(
            "Failed to create ~/.local/bin in WSL: {}",
            String::from_utf8_lossy(&mkdir_output.stderr)
        ));
    }

    // Copy binary to WSL
    let cp_output = Command::new("wsl")
        .args(["cp", &wsl_source_path, "~/.local/bin/kvelmo"])
        .output()
        .map_err(|e| format!("Failed to copy binary to WSL: {}", e))?;

    if !cp_output.status.success() {
        return Err(format!(
            "Failed to copy kvelmo to WSL: {}",
            String::from_utf8_lossy(&cp_output.stderr)
        ));
    }

    // Make executable
    let chmod_output = Command::new("wsl")
        .args(["chmod", "+x", "~/.local/bin/kvelmo"])
        .output()
        .map_err(|e| format!("Failed to chmod in WSL: {}", e))?;

    if !chmod_output.status.success() {
        return Err(format!(
            "Failed to make kvelmo executable: {}",
            String::from_utf8_lossy(&chmod_output.stderr)
        ));
    }

    // Verify installation
    let verify_output = Command::new("wsl")
        .args(["~/.local/bin/kvelmo", "--version"])
        .output()
        .map_err(|e| format!("Failed to verify installation: {}", e))?;

    if !verify_output.status.success() {
        return Err("kvelmo installation verification failed".to_string());
    }

    let version = String::from_utf8_lossy(&verify_output.stdout);
    tracing::info!("Successfully deployed kvelmo to WSL: {}", version.trim());

    Ok(())
}

/// Convert a Windows path to a WSL path
/// e.g., C:\Users\foo\bar -> /mnt/c/Users/foo/bar
pub fn windows_to_wsl_path(win_path: &str) -> String {
    // Handle UNC paths and regular paths
    let path = win_path.replace('\\', "/");

    // Check for drive letter pattern (e.g., C:/)
    if path.len() >= 2 && path.chars().nth(1) == Some(':') {
        let drive = path.chars().next().unwrap().to_ascii_lowercase();
        let rest = &path[2..]; // Skip "C:"
        let rest = rest.strip_prefix('/').unwrap_or(rest);
        format!("/mnt/{}/{}", drive, rest)
    } else {
        path
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_windows_to_wsl_path() {
        assert_eq!(
            windows_to_wsl_path(r"C:\Users\foo\bar"),
            "/mnt/c/Users/foo/bar"
        );
        assert_eq!(
            windows_to_wsl_path(r"D:\Projects\kvelmo"),
            "/mnt/d/Projects/kvelmo"
        );
        assert_eq!(
            windows_to_wsl_path("C:/Users/foo/bar"),
            "/mnt/c/Users/foo/bar"
        );
    }

    #[test]
    fn test_windows_to_wsl_path_lowercase_drive() {
        assert_eq!(windows_to_wsl_path(r"c:\users\test"), "/mnt/c/users/test");
    }

    #[test]
    fn test_windows_to_wsl_path_root_only() {
        assert_eq!(windows_to_wsl_path(r"C:\"), "/mnt/c/");
    }

    #[test]
    fn test_windows_to_wsl_path_spaces() {
        assert_eq!(
            windows_to_wsl_path(r"C:\Program Files\My App"),
            "/mnt/c/Program Files/My App"
        );
    }

    #[test]
    fn test_windows_to_wsl_path_no_drive_passthrough() {
        assert_eq!(windows_to_wsl_path("relative/path"), "relative/path");
    }

    #[test]
    fn test_windows_to_wsl_path_empty() {
        assert_eq!(windows_to_wsl_path(""), "");
    }

    #[test]
    fn test_windows_to_wsl_path_mixed_slashes() {
        assert_eq!(
            windows_to_wsl_path(r"C:\Users/foo\bar"),
            "/mnt/c/Users/foo/bar"
        );
    }

    #[test]
    fn test_windows_to_wsl_path_unicode() {
        assert_eq!(
            windows_to_wsl_path(r"C:\Users\用户\ドキュメント"),
            "/mnt/c/Users/用户/ドキュメント"
        );
    }

    #[test]
    fn test_windows_to_wsl_path_very_long() {
        let long_segment = "a".repeat(200);
        let input = format!(r"C:\{}\{}", long_segment, long_segment);
        let expected = format!("/mnt/c/{}/{}", long_segment, long_segment);
        assert_eq!(windows_to_wsl_path(&input), expected);
    }

    #[test]
    fn test_windows_to_wsl_path_drive_letter_only() {
        assert_eq!(windows_to_wsl_path("C:"), "/mnt/c/");
    }

    #[test]
    fn test_windows_to_wsl_path_double_backslash() {
        assert_eq!(
            windows_to_wsl_path(r"C:\\Users\\test"),
            "/mnt/c//Users//test"
        );
    }

    #[test]
    fn test_windows_to_wsl_path_trailing_backslash() {
        assert_eq!(windows_to_wsl_path(r"C:\Users\foo\"), "/mnt/c/Users/foo/");
    }

    #[test]
    fn test_windows_to_wsl_path_forward_slash_no_drive() {
        assert_eq!(windows_to_wsl_path("/usr/local/bin"), "/usr/local/bin");
    }

    #[test]
    fn test_windows_to_wsl_path_single_char_not_drive() {
        assert_eq!(windows_to_wsl_path("X"), "X");
    }
}
