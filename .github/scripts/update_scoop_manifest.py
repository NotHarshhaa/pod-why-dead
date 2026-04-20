#!/usr/bin/env python3
import sys
import json
import re

def update_scoop_manifest(version, checksums_file, manifest_file):
    # Read checksums
    checksums = {}
    with open(checksums_file, 'r') as f:
        for line in f:
            parts = line.strip().split()
            if len(parts) >= 2:
                checksums[parts[1]] = parts[0]
    
    # Read manifest
    with open(manifest_file, 'r') as f:
        manifest = json.load(f)
    
    # Update version
    manifest['version'] = version
    
    # Update URL
    filename = f"pod-why-dead_{version}_windows_amd64.zip"
    manifest['architecture']['64bit']['url'] = f"https://github.com/NotHarshhaa/pod-why-dead/releases/download/v{version}/{filename}"
    
    # Update hash
    if filename in checksums:
        manifest['architecture']['64bit']['hash'] = checksums[filename]
    
    # Write back
    with open(manifest_file, 'w') as f:
        json.dump(manifest, f, indent=2)
    
    print(f"Updated Scoop manifest to version {version}")

if __name__ == "__main__":
    if len(sys.argv) != 4:
        print("Usage: update_scoop_manifest.py <version> <checksums_file> <manifest_file>")
        sys.exit(1)
    
    update_scoop_manifest(sys.argv[1], sys.argv[2], sys.argv[3])
