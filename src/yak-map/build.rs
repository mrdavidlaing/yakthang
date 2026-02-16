use std::process::Command;

fn main() {
    let date = Command::new("date")
        .args(["+%Y%m%d"])
        .output()
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .unwrap_or_else(|_| "unknown".to_string());

    let git_sha = Command::new("git")
        .args(["rev-parse", "--short", "HEAD"])
        .output()
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .unwrap_or_else(|_| "unknown".to_string());

    let version = format!("{}-{}", date, git_sha);
    println!("cargo:rustc-env=VERSION={}", version);
}
