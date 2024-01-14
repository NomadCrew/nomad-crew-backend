import os
import glob
import tkinter as tk
from tkinter import filedialog, messagebox, ttk

def compile_files(directory, extensions, output_file):
    os.chdir(directory)
    with open(output_file, 'w') as outfile:
        for ext in extensions:
            for filepath in glob.glob(f'**/*.{ext}', recursive=True):
                outfile.write(f"{filepath}```\n")
                with open(filepath, 'r') as infile:
                    content = infile.read()
                    outfile.write(content)
                outfile.write("\n```\n")
    return output_file

def on_submit():
    selected_extensions = [ext.get() for ext in extension_vars if ext.get()]
    if not selected_extensions:
        messagebox.showwarning("Warning", "Please select at least one file extension.")
        return
    if not source_dir.get():
        messagebox.showwarning("Warning", "Please select a source directory.")
        return
    if not output_file.get():
        messagebox.showwarning("Warning", "Please specify an output file name.")
        return
    
    compiled_file = compile_files(source_dir.get(), selected_extensions, output_file.get())
    messagebox.showinfo("Success", f"Files compiled successfully into {compiled_file}")

# GUI setup
root = tk.Tk()
root.title("File Compiler")
root.geometry("400x300")

# Styling
style = ttk.Style()
style.theme_use('clam')

# Source directory selection
source_dir = tk.StringVar()
ttk.Label(root, text="Source Directory:").pack(pady=(10,0))
source_entry = ttk.Entry(root, textvariable=source_dir, width=50)
source_entry.pack()
ttk.Button(root, text="Browse", command=lambda: source_dir.set(filedialog.askdirectory())).pack(pady=(0,10))

# File extension options
extension_frame = ttk.LabelFrame(root, text="Select File Extensions")
extension_frame.pack(pady=10)
extension_vars = []
for ext in ['py', 'go', 'java', 'js', 'txt']:  # Add more extensions as needed
    var = tk.StringVar(value=ext)
    chk = ttk.Checkbutton(extension_frame, text=ext.upper(), variable=var, onvalue=ext, offvalue="")
    chk.pack(side='left', padx=5, pady=5)
    extension_vars.append(var)

# Output file name
output_file = tk.StringVar()
ttk.Label(root, text="Output File Name:").pack()
output_entry = ttk.Entry(root, textvariable=output_file, width=50)
output_entry.pack(pady=(0,10))

# Submit button
submit_button = ttk.Button(root, text="Compile Files", command=on_submit)
submit_button.pack(pady=10)

# Run the GUI
root.mainloop()
