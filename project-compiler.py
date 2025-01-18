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

        # Calculate file count, including special ignores
        for root, _, files in os.walk(directory):
            if 'charting_library' in root or 'datafeed' in root or is_ignored(root, spec, directory):
                continue

            for file in files:
                filepath = os.path.join(root, file)

                if not ('charting_library' in filepath or 'datafeed' in filepath) and not is_ignored(filepath, spec, directory):
                    if file != 'package-lock.json' and file != 'yarn.lock' and any(file.endswith(f".{ext}") for ext in extensions):
                        file_count += 1

        file_count += 2  # For Dockerfile and docker-compose.yml

        progress_step = 100 / max(file_count, 1)
        progress = 0

        with open(output_file, 'w', encoding='utf-8') as outfile:
            for root, _, files in os.walk(directory):
                if 'charting_library' in root or 'datafeed' in root or is_ignored(root, spec, directory):
                    continue

                for file in files:
                    filepath = os.path.join(root, file)
                    if 'charting_library' in filepath or 'datafeed' in filepath or is_ignored(filepath, spec, directory):
                        continue

                    if file == 'package-lock.json' or file == 'yarn.lock':
                        continue
                    if any(file.endswith(f".{ext}") for ext in extensions):
                        outfile.write(f"{filepath}```\n")
                        with open(filepath, 'r', encoding='utf-8', errors='ignore') as infile:
                            outfile.write(infile.read())
                        outfile.write("\n```\n")
                        progress += progress_step
                        if progress_callback:
                            progress_callback(progress)

            for dockerfile in ['Dockerfile', 'docker-compose.yml']:
                dockerpath = os.path.join(directory, dockerfile)

                if os.path.exists(dockerpath) and not is_ignored(dockerpath, spec, directory):
                    outfile.write(f"{dockerfile}```\n")
                    with open(dockerpath, 'r', encoding='utf-8', errors='ignore') as infile:
                        outfile.write(infile.read())
                    outfile.write("\n```\n")
                    progress += progress_step
                    if progress_callback:
                        progress_callback(progress)

        if progress_callback:
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
for ext in ['py', 'go', 'java', 'js', 'txt', 'sql', 'ts', 'tsx', 'jsx', 'html', 'css', 'json', 'toml']:  # Add more extensions as needed
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
