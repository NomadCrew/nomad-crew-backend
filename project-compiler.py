import os
import pathspec
import tkinter as tk
from tkinter import filedialog, messagebox, ttk


def compile_files(directory, extensions, output_file, progress_callback=None):
    def load_gitignore(directory):
        gitignore_path = os.path.join(directory, ".gitignore")
        if os.path.exists(gitignore_path):
            with open(gitignore_path, "r", encoding="utf-8") as gitignore_file:
                return pathspec.PathSpec.from_lines("gitwildmatch", gitignore_file)
        return None

    def is_ignored(path, spec, directory):
        if spec:
            relative_path = os.path.relpath(path, directory)
            return spec.match_file(relative_path)
        return False

    try:
        spec = load_gitignore(directory)
        file_count = 0
        all_files = []  # New list to track collected files

        # First pass to collect files and count
        for root, _, files in os.walk(directory):
            if 'charting_library' in root or 'datafeed' in root or is_ignored(root, spec, directory):
                continue

            for file in files:
                filepath = os.path.join(root, file)

                if not ('charting_library' in filepath or 'datafeed' in filepath) and not is_ignored(filepath, spec, directory):
                    if file != 'package-lock.json' and file != 'yarn.lock' and any(file.endswith(f".{ext}") for ext in extensions):
                        file_count += 1
                        all_files.append(filepath)  # Track file path

        # Collect Dockerfiles
        docker_files = []
        for dockerfile in ['Dockerfile', 'docker-compose.yml']:
            dockerpath = os.path.join(directory, dockerfile)
            if os.path.exists(dockerpath) and not is_ignored(dockerpath, spec, directory):
                file_count += 1
                all_files.append(dockerpath)
                docker_files.append(dockerpath)

        # New function to generate tree structure
        def generate_project_tree(files, root_dir):
            tree = {}
            for path in files:
                rel_path = os.path.relpath(path, root_dir)
                parts = rel_path.split(os.sep)
                current = tree
                for part in parts[:-1]:
                    current = current.setdefault(part + '/', {})
                current[parts[-1]] = None
            return tree

        # Generate tree string
        def format_tree(tree, indent=''):
            lines = []
            for i, (name, children) in enumerate(sorted(tree.items())):
                is_last = i == len(tree)-1
                prefix = '└── ' if is_last else '├── '
                lines.append(f"{indent}{prefix}{name}")
                if children:
                    new_indent = indent + ('    ' if is_last else '│   ')
                    lines.extend(format_tree(children, new_indent))
            return lines

        # Generate the full tree output
        tree_structure = [os.path.basename(directory) + '/']
        tree_structure.extend(format_tree(generate_project_tree(all_files, directory)))
        tree_output = '\n'.join(tree_structure)

        progress_step = 100 / max(file_count, 1)
        progress = 0

        with open(output_file, 'w', encoding='utf-8') as outfile:
            # Write project tree at the top
            outfile.write(f"PROJECT TREE:\n{tree_output}\n\n{'='*50}\n\n")

            for filepath in all_files:
                outfile.write(f"{filepath}```\n")
                with open(filepath, 'r', encoding='utf-8', errors='ignore') as infile:
                    outfile.write(infile.read())
                outfile.write("\n```\n")
                progress += progress_step
                if progress_callback:
                    progress_callback(min(progress, 100))

            if progress_callback and file_count == 0:
                progress_callback(100)
        return output_file
    except Exception as e:
        return f"Error: {str(e)}"


def on_submit():
    selected_extensions = [extension_list.get(
        i) for i in extension_list.curselection()]
    if not selected_extensions:
        messagebox.showwarning(
            "Warning", "Please select at least one file extension.")
        return
    if not source_dir.get():
        messagebox.showwarning("Warning", "Please select a source directory.")
        return
    if not output_file.get():
        messagebox.showwarning(
            "Warning", "Please specify an output file name.")
        return

    # Ensure output file is saved in the desired directory
    output_directory = r"N:\NomadCrew\Co-pilot assets"
    output_path = os.path.join(output_directory, output_file.get())

    def update_progress(value):
        progress_bar["value"] = value
        root.update_idletasks()

    compiled_file = compile_files(
        source_dir.get(), selected_extensions, output_path, update_progress)
    if "Error" in compiled_file:
        messagebox.showerror("Error", compiled_file)
    else:
        messagebox.showinfo(
            "Success", f"Files compiled successfully into {compiled_file}")
        progress_bar["value"] = 0

# GUI setup


root = tk.Tk()
root.title("File Compiler")
root.geometry("500x400")

# Styling
style = ttk.Style()
style.theme_use('clam')

# Source directory selection
source_dir = tk.StringVar()
ttk.Label(root, text="Source Directory:").pack(pady=(10, 0))
source_entry = ttk.Entry(root, textvariable=source_dir, width=60)
source_entry.pack()
ttk.Button(root, text="Browse", command=lambda: source_dir.set(
    filedialog.askdirectory())).pack(pady=(0, 10))

# File extension options
extension_frame = ttk.LabelFrame(root, text="Select File Extensions")
extension_frame.pack(pady=10, fill="both", expand=True)
extension_list = tk.Listbox(
    extension_frame, selectmode='multiple', height=6, width=30)
for ext in ['py', 'go', 'java', 'js', 'txt', 'sql', 'ts', 'tsx', 'jsx', 'html', 'css', 'json', 'toml', 'yaml']:  # Add more extensions as needed
    extension_list.insert(tk.END, ext)
extension_list.pack(fill='both', expand=True)

# Output file name
output_file = tk.StringVar()
ttk.Label(root, text="Output File Name:").pack()
output_entry = ttk.Entry(root, textvariable=output_file, width=60)
output_entry.pack(pady=(0, 10))

# Progress bar
progress_bar = ttk.Progressbar(
    root, orient='horizontal', mode='determinate', length=400)
progress_bar.pack(pady=10)

# Submit button
submit_button = ttk.Button(root, text="Compile Files", command=on_submit)
submit_button.pack(pady=10)

# Run the GUI
root.mainloop()
