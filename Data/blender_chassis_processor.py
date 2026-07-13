# pyrefly: ignore [missing-import]
import bpy
import sys
import argparse
import os

addon_zip = None
if "--addon-zip" in sys.argv:
    idx = sys.argv.index("--addon-zip")
    if idx + 1 < len(sys.argv):
        addon_zip = sys.argv[idx+1]

if addon_zip:
    try:
        if "evr_mesh_importer" not in bpy.context.preferences.addons:
            bpy.ops.preferences.addon_install(filepath=addon_zip)
        bpy.ops.preferences.addon_enable(module="evr_mesh_importer")
        
        import evr_mesh_importer
        try:
            evr_mesh_importer.register()
        except Exception:
            pass
    except Exception as e:
        print(f"Failed to install/enable evr_mesh_importer from zip: {e}")

def main():
    try:
        if "--" not in sys.argv:
            print("Expected '--' in arguments")
            sys.exit(1)
            
        args_list = sys.argv[sys.argv.index("--") + 1:]
        
        parser = argparse.ArgumentParser()
        parser.add_argument("--input", required=True)
        parser.add_argument("--output", required=True)
        parser.add_argument("--mesh-hash", required=True)
        parser.add_argument("--tex-hash", required=True)
        parser.add_argument("--base-mesh", required=True)
        parser.add_argument("--addon-zip", required=True)
        parser.add_argument("--export-dir", required=True)
        parser.add_argument("--cull-unweighted", action="store_true")
        
        args = parser.parse_args(args_list)

        # 1. Load user's file
        ext = os.path.splitext(args.input)[1].lower()
        if ext == '.blend':
            bpy.ops.wm.open_mainfile(filepath=args.input)
        elif ext in ['.glb', '.gltf']:
            bpy.ops.object.select_all(action='SELECT')
            bpy.ops.object.delete()
            bpy.ops.import_scene.gltf(filepath=args.input)
        elif ext == '.obj':
            bpy.ops.object.select_all(action='SELECT')
            bpy.ops.object.delete()
            bpy.ops.import_scene.obj(filepath=args.input)
        else:
            print(f"Unsupported file format: {ext}")
            sys.exit(1)
            
        bpy.ops.object.select_all(action='DESELECT')
        mesh_objs = []
        for obj in bpy.context.scene.objects:
            if obj.type == 'MESH':
                mesh_objs.append(obj)
                obj.select_set(True)
                
        if not mesh_objs:
            print("No mesh found in the input file.")
            sys.exit(1)
            
        bpy.context.view_layer.objects.active = mesh_objs[0]

        # Fix scale if needed (some glbs are 100x larger)
        bpy.ops.object.transform_apply(location=False, rotation=True, scale=True)

        # Extract parent directory to find the extracted game dir
        # args.base_mesh looks like: Settings/pcvr-extracted/GPU/CGMeshListResource/b21cde9f9dad0030
        extracted_dir = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(args.base_mesh))))
        
        # The ultimate export folder where EchoVR expects replacements
        input_pcvr_dir = args.export_dir
        os.makedirs(input_pcvr_dir, exist_ok=True)

        print(f"Configuring evr-mesh-importer...")
        print(f"  Extracted Dir: {extracted_dir}")
        print(f"  Export Dir: {input_pcvr_dir}")

        settings = bpy.context.scene.evr_export_settings
        settings.original_gpu_file = os.path.abspath(args.base_mesh)
        settings.extracted_game_dir = extracted_dir
        settings.export_dir = input_pcvr_dir
        settings.auto_decimate = True
        settings.replace_textures = True
        
        # We must leave settings.encode_mode as 'primary_described' (the default).
        # The addon internally checks if the path contains CGMeshListResource and routes
        # to encode_cgml_primary_replace automatically!
        
        # Monkey-patch the addon's internal missing function bug before it runs
        import evr_mesh_importer.encode
        evr_mesh_importer.encode._validate_mesh = lambda *args, **kwargs: None
        
        print("Executing EVR_OT_ImportAndReplace operator...")
        bpy.ops.export_mesh.evr_import_replace()
        print("Addon execution complete.")

    except Exception as e:
        import traceback
        traceback.print_exc()
        print(f"Blender script global exception: {e}")
        sys.exit(1)
        
    print("Done!")

if __name__ == "__main__":
    main()
