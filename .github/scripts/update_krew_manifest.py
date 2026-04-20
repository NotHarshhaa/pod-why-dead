#!/usr/bin/env python3
import sys
import re

def update_manifest(version, checksums_file, manifest_file):
    # Read checksums
    checksums = {}
    with open(checksums_file, 'r') as f:
        for line in f:
            parts = line.strip().split()
            if len(parts) >= 2:
                checksums[parts[1]] = parts[0]
    
    # Read manifest
    with open(manifest_file, 'r') as f:
        content = f.read()
    
    # Update version
    content = re.sub(r'version: ".*"', f'version: "{version}"', content)
    
    # Update URLs and checksums
    platforms = {
        'linux_amd64': 'linux',
        'linux_arm64': 'linux',
        'darwin_amd64': 'darwin',
        'darwin_arm64': 'darwin',
        'windows_amd64': 'windows'
    }
    
    for platform, os_name in platforms.items():
        # Find the file in checksums
        filename = f"pod-why-dead_{version}_{os_name}_{platform.split('_')[1]}.tar.gz"
        if os_name == 'windows':
            filename = f"pod-why-dead_{version}_{os_name}_{platform.split('_')[1]}.zip"
        
        if filename in checksums:
            checksum = checksums[filename]
            # Update URL
            old_url = f"https://github.com/NotHarshhaa/pod-why-dead/releases/download/[^/]+/{filename}"
            new_url = f"https://github.com/NotHarshhaa/pod-why-dead/releases/download/{version}/{filename}"
            content = re.sub(old_url, new_url, content)
            
            # Update checksum
            # Find the sha256 line for this platform
            pattern = rf'(arch: {platform.split("_")[1]}\s+uri:.*?sha256: )"([^"]*)"'
            replacement = rf'\g<1>{checksum}'
            content = re.sub(pattern, replacement, content, flags=re.DOTALL)
    
    # Write back
    with open(manifest_file, 'w') as f:
        f.write(content)
    
    print(f"Updated manifest to version {version}")

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: update_krew_manifest.py <version> <checksums_file> <manifest_file>")
        sys.exit(1)
    
    update_manifest(sys.argv[1], sys.argv[2], sys.argv[3])
