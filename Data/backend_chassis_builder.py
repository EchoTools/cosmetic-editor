import os
import sys
import subprocess
import json
import uuid
import random
import shutil
import glob
import argparse

def main():
    parser = argparse.ArgumentParser(description="Build custom chassis")
    parser.add_argument("input_blend")
    parser.add_argument("output_dir")
    parser.add_argument("--mesh-hash", type=int, default=None)
    parser.add_argument("--rig-hash", type=int, default=None)
    parser.add_argument("--mat-hash", type=int, default=None)
    parser.add_argument("--tex-hash", type=int, default=None)
    parser.add_argument("--base-mesh-3p", type=str, default=None)
    parser.add_argument("--base-mesh-1p", type=str, default=None)
    parser.add_argument("--addon-zip", type=str, default=None)
    parser.add_argument("--type", type=str, choices=['chassis', 'booster', 'bracer'], default='chassis')
    parser.add_argument("--target", type=str, choices=['3p', '1p', 'both'], default='both')
    parser.add_argument("--export-dir", type=str, default=None)
    
    args = parser.parse_args()

    input_blend = os.path.abspath(args.input_blend)
    output_dir = os.path.abspath(args.output_dir)

    if not os.path.exists(output_dir):
        os.makedirs(output_dir)

    script_dir = os.path.dirname(os.path.abspath(__file__))
    blender_script = os.path.join(script_dir, "blender_chassis_processor.py")

    # Hashes:
    mesh_hash_uint = (args.mesh_hash & 0xFFFFFFFFFFFFFFFF) if args.mesh_hash is not None else random.getrandbits(63)
    rig_hash_uint = (args.rig_hash & 0xFFFFFFFFFFFFFFFF) if args.rig_hash is not None else random.getrandbits(63)
    mat_hash_uint = (args.mat_hash & 0xFFFFFFFFFFFFFFFF) if args.mat_hash is not None else random.getrandbits(63)
    tex_hash_uint = (args.tex_hash & 0xFFFFFFFFFFFFFFFF) if args.tex_hash is not None else random.getrandbits(63)

    blender_exe = shutil.which("blender")
    if not blender_exe:
        # Try finding it in typical Windows install paths
        possible_paths = glob.glob(r"C:\Program Files\Blender Foundation\Blender *\blender.exe")
        if possible_paths:
            # Sort to get the latest version
            possible_paths.sort(reverse=True)
            blender_exe = possible_paths[0]
            
    if not blender_exe:
        print("BLENDER_NOT_FOUND")
        sys.exit(99)

    if not args.addon_zip or not os.path.exists(args.addon_zip):
        print(f"Addon zip not found: {args.addon_zip}")
        sys.exit(1)

    # 3rd Person Export (Or Booster/Bracer Export)
    if args.target in ['3p', 'both']:
        blender_args_3p = [
            blender_exe, "-b", "--python", blender_script, "--",
            "--input", input_blend,
            "--output", output_dir,
            "--mesh-hash", f"{mesh_hash_uint:016x}",
            "--tex-hash", f"{tex_hash_uint:016x}"
        ]
        
        if args.addon_zip is not None:
            blender_args_3p.extend(["--addon-zip", args.addon_zip])
        if args.base_mesh_3p is not None:
            blender_args_3p.extend(["--base-mesh", args.base_mesh_3p])
        if args.export_dir is not None:
            blender_args_3p.extend(["--export-dir", args.export_dir])

        print(f"Running blender for 3rd Person: {' '.join(blender_args_3p)}")
        result_3p = subprocess.run(blender_args_3p, capture_output=True, text=True)
        if result_3p.returncode != 0 or "Traceback" in result_3p.stdout or "Traceback" in result_3p.stderr or "Exception" in result_3p.stdout or "Exception" in result_3p.stderr:
            print("Blender 3rd Person Output:")
            print(result_3p.stdout)
            print(result_3p.stderr, file=sys.stderr)
            sys.exit(1 if result_3p.returncode == 0 else result_3p.returncode)

    # 1st Person Export (Arms Only) - Only for chassis!
    if args.type == 'chassis' and args.target in ['1p', 'both']:
        blender_args_1p = [
            blender_exe, "-b", "--python", blender_script, "--",
            "--input", input_blend,
            "--output", output_dir,
            "--mesh-hash", f"{rig_hash_uint:016x}",
            "--tex-hash", f"{tex_hash_uint:016x}",
            "--cull-unweighted"
        ]
        
        if args.addon_zip is not None:
            blender_args_1p.extend(["--addon-zip", args.addon_zip])
        if args.base_mesh_1p is not None:
            blender_args_1p.extend(["--base-mesh", args.base_mesh_1p])
        if args.export_dir is not None:
            blender_args_1p.extend(["--export-dir", args.export_dir])
        
        print(f"Running blender for 1st Person: {' '.join(blender_args_1p)}")
        result_1p = subprocess.run(blender_args_1p, capture_output=True, text=True)
        if result_1p.returncode != 0 or "Traceback" in result_1p.stdout or "Traceback" in result_1p.stderr or "Exception" in result_1p.stdout or "Exception" in result_1p.stderr:
            print("Blender 1st Person Output:")
            print(result_1p.stdout)
            print(result_1p.stderr, file=sys.stderr)
            sys.exit(1 if result_1p.returncode == 0 else result_1p.returncode)
        
    manifest_path = os.path.join(output_dir, "manifest.json")
    manifest = {
        "AssetSymbol5": mesh_hash_uint,
        "AssetSymbol6": rig_hash_uint if args.type == 'chassis' else 0,
        "AssetSymbol11": mat_hash_uint if args.type == 'chassis' else 0,
        "AssetSymbol12": tex_hash_uint,
        "MeshHashHex": f"{mesh_hash_uint:016x}",
        "TextureHashHex": f"{tex_hash_uint:016x}",
        "MeshFileGpu": os.path.join(output_dir, f"{mesh_hash_uint:016x}_gpu"),
        "MeshFilePri": os.path.join(output_dir, f"{mesh_hash_uint:016x}_pri"),
        "TextureFile": os.path.join(output_dir, f"{tex_hash_uint:016x}.png")
    }
    
    if args.type == 'chassis':
        manifest["RigHashHex"] = f"{rig_hash_uint:016x}"
        manifest["RigFileGpu"] = os.path.join(output_dir, f"{rig_hash_uint:016x}_gpu")
        manifest["RigFilePri"] = os.path.join(output_dir, f"{rig_hash_uint:016x}_pri")

    with open(manifest_path, 'w') as f:
        json.dump(manifest, f, indent=4)

    print(f"Manifest written to: {manifest_path}")

if __name__ == "__main__":
    main()
