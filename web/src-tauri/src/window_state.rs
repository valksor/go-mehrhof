//! Window state persistence
//!
//! Saves and restores window position, size, and maximized state
//! to ~/.valksor/kvelmo/window-state.json

use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;
use tauri::{PhysicalPosition, PhysicalSize, WebviewWindow, Window};

/// Persisted window state
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WindowState {
    pub x: i32,
    pub y: i32,
    pub width: u32,
    pub height: u32,
    pub is_maximized: bool,
}

impl WindowState {
    /// Get the path to the window state file
    fn state_path() -> Option<PathBuf> {
        dirs::home_dir().map(|h| h.join(".valksor").join("kvelmo").join("window-state.json"))
    }

    /// Load window state from disk
    pub fn load() -> Option<Self> {
        let path = Self::state_path()?;
        let content = fs::read_to_string(path).ok()?;
        serde_json::from_str(&content).ok()
    }

    /// Save window state to disk
    pub fn save(&self) -> Result<(), String> {
        let path = Self::state_path().ok_or("Could not determine state path")?;

        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }

        let content = serde_json::to_string_pretty(self).map_err(|e| e.to_string())?;
        fs::write(path, content).map_err(|e| e.to_string())
    }
}

/// Restore window state from saved file
pub fn restore(window: &WebviewWindow) {
    if let Some(state) = WindowState::load() {
        // Restore position
        let _ = window.set_position(PhysicalPosition::new(state.x, state.y));

        // Restore size
        let _ = window.set_size(PhysicalSize::new(state.width, state.height));

        // Restore maximized state
        if state.is_maximized {
            let _ = window.maximize();
        }
    }
}

/// Save current window state to file (from Window in event handler)
pub fn save(window: &Window) {
    let Ok(position) = window.outer_position() else {
        return;
    };
    let Ok(size) = window.outer_size() else {
        return;
    };

    let state = WindowState {
        x: position.x,
        y: position.y,
        width: size.width,
        height: size.height,
        is_maximized: window.is_maximized().unwrap_or(false),
    };

    if let Err(e) = state.save() {
        tracing::warn!("Failed to save window state: {}", e);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn test_window_state_serialization_roundtrip() {
        let state = WindowState {
            x: 100,
            y: 200,
            width: 1200,
            height: 800,
            is_maximized: false,
        };
        let json = serde_json::to_string(&state).unwrap();
        let restored: WindowState = serde_json::from_str(&json).unwrap();

        assert_eq!(restored.x, 100);
        assert_eq!(restored.y, 200);
        assert_eq!(restored.width, 1200);
        assert_eq!(restored.height, 800);
        assert!(!restored.is_maximized);
    }

    #[test]
    fn test_window_state_negative_coordinates() {
        // Multi-monitor setups can have negative positions
        let state = WindowState {
            x: -1920,
            y: -100,
            width: 1200,
            height: 800,
            is_maximized: false,
        };
        let json = serde_json::to_string(&state).unwrap();
        let restored: WindowState = serde_json::from_str(&json).unwrap();

        assert_eq!(restored.x, -1920);
        assert_eq!(restored.y, -100);
    }

    #[test]
    fn test_window_state_maximized() {
        let json = r#"{"x":0,"y":0,"width":1920,"height":1080,"is_maximized":true}"#;
        let state: WindowState = serde_json::from_str(json).unwrap();
        assert!(state.is_maximized);
    }

    #[test]
    fn test_save_creates_parent_directories() {
        let temp_dir = TempDir::new().unwrap();
        let nested_path = temp_dir.path().join("nested").join("dir");
        fs::create_dir_all(&nested_path).unwrap();
        assert!(nested_path.exists());
    }

    #[test]
    fn test_load_handles_corrupt_json() {
        let temp_dir = TempDir::new().unwrap();
        let path = temp_dir.path().join("corrupt.json");
        fs::write(&path, "{ invalid json }").unwrap();

        let content = fs::read_to_string(&path).unwrap();
        let result: Result<WindowState, _> = serde_json::from_str(&content);
        assert!(result.is_err());
    }

    #[test]
    fn test_deserialize_with_extra_fields() {
        let json =
            r#"{"x":10,"y":20,"width":800,"height":600,"is_maximized":false,"new_field":"hello"}"#;
        let state: WindowState = serde_json::from_str(json).unwrap();
        assert_eq!(state.x, 10);
        assert_eq!(state.width, 800);
    }

    #[test]
    fn test_deserialize_missing_field_fails() {
        let json = r#"{"x":10,"y":20,"width":800}"#;
        let result: Result<WindowState, _> = serde_json::from_str(json);
        assert!(result.is_err());
    }

    #[test]
    fn test_zero_size_window() {
        let state = WindowState {
            x: 0,
            y: 0,
            width: 0,
            height: 0,
            is_maximized: false,
        };
        let json = serde_json::to_string(&state).unwrap();
        let restored: WindowState = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.width, 0);
        assert_eq!(restored.height, 0);
    }

    #[test]
    fn test_very_large_dimensions() {
        let state = WindowState {
            x: i32::MAX,
            y: i32::MIN,
            width: u32::MAX,
            height: u32::MAX,
            is_maximized: true,
        };
        let json = serde_json::to_string(&state).unwrap();
        let restored: WindowState = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.x, i32::MAX);
        assert_eq!(restored.y, i32::MIN);
        assert_eq!(restored.width, u32::MAX);
    }

    #[test]
    fn test_file_io_roundtrip() {
        let temp_dir = TempDir::new().unwrap();
        let path = temp_dir.path().join("state.json");

        let state = WindowState {
            x: 50,
            y: 75,
            width: 1024,
            height: 768,
            is_maximized: true,
        };
        let content = serde_json::to_string_pretty(&state).unwrap();
        fs::write(&path, &content).unwrap();

        let loaded_content = fs::read_to_string(&path).unwrap();
        let loaded: WindowState = serde_json::from_str(&loaded_content).unwrap();
        assert_eq!(loaded.x, 50);
        assert_eq!(loaded.y, 75);
        assert_eq!(loaded.width, 1024);
        assert_eq!(loaded.height, 768);
        assert!(loaded.is_maximized);
    }

    #[test]
    fn test_load_nonexistent_file_returns_error() {
        let result = fs::read_to_string("/tmp/definitely_does_not_exist_kvelmo_test.json");
        assert!(result.is_err());
    }

    #[test]
    fn test_load_empty_file_fails_parse() {
        let temp_dir = TempDir::new().unwrap();
        let path = temp_dir.path().join("empty.json");
        fs::write(&path, "").unwrap();

        let content = fs::read_to_string(&path).unwrap();
        let result: Result<WindowState, _> = serde_json::from_str(&content);
        assert!(result.is_err());
    }

    #[test]
    fn test_deserialize_null_values_fails() {
        let json = r#"{"x":null,"y":0,"width":800,"height":600,"is_maximized":false}"#;
        let result: Result<WindowState, _> = serde_json::from_str(json);
        assert!(result.is_err());
    }
}
