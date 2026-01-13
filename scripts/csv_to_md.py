#!/usr/bin/env python3
import csv
import sys
from collections import defaultdict

def main():
    # Read CSV data
    # data[bench_type][library][size] = metrics
    data = defaultdict(lambda: defaultdict(dict))
    
    try:
        with open('bench.csv', 'r') as f:
            reader = csv.DictReader(f)
            for row in reader:
                bench_type = row.get('type', '')
                library = row.get('library', '')
                if not bench_type or not library or not row.get('size'):
                    continue
                size = int(row['size'])
                data[bench_type][library][size] = {
                    'ns_per_op': row['ns_per_op'],
                    'mb_per_s': row['mb_per_s'],
                    'bytes_per_op': row['bytes_per_op'],
                    'allocs_per_op': row['allocs_per_op']
                }
    except FileNotFoundError:
        print("Error: bench.csv not found.")
        sys.exit(1)

    if not data:
        print("Error: No data found in bench.csv.")
        sys.exit(1)

    # Generate markdown
    output = []
    output.append("# Benchmarks")
    output.append("")
    output.append(r"Command used: \`FFT_BENCH_MAX=32768 GOAMD64=v3 go test -tags=asm -bench . -benchmem ./bench\`")
    output.append("")
    output.append("Notes:")
    output.append("- Results are from the latest local run.")
    output.append(r"- \`algo-fft\` benchmarks include both complex128 and complex64.")
    output.append(r"- \`go-fftw\` requires FFTW shared libraries.")
    output.append(r"- \`go-dsp-fft\` allocates on every call (no reusable plan).")
    output.append("")
    
    # Sort benchmark types: FFT, IFFT, FFT32, IFFT32
    type_order = {'FFT': 0, 'IFFT': 1, 'FFT32': 2, 'IFFT32': 3}
    sorted_types = sorted(data.keys(), key=lambda t: type_order.get(t, 99))

    for bench_type in sorted_types:
        output.append(f"## {bench_type} Benchmarks")
        output.append("")
        
        libraries = sorted(data[bench_type].keys())
        for library in libraries:
            output.append(f"### {library}")
            output.append("")
            output.append("|  Size |   ns/op |    MB/s |    B/op | allocs/op |")
            output.append("| ----- | ------- | ------- | ------- | --------- |")
            
            sizes = sorted(data[bench_type][library].keys())
            for size in sizes:
                row = data[bench_type][library][size]
                ns_op = row['ns_per_op']
                mb_s = row['mb_per_s']
                b_op = row['bytes_per_op']
                allocs = row['allocs_per_op']
                
                # Format numbers nicely
                try:
                    ns_val = float(ns_op)
                    ns_op = f"{ns_val:.1f}" if ns_val < 1000 else f"{int(ns_val)}"
                except ValueError: pass
                
                try:
                    mb_val = float(mb_s)
                    mb_s = f"{mb_val:.2f}"
                except ValueError: pass
                
                output.append(f"| {size:5} | {ns_op:>7} | {mb_s:>7} | {b_op:>7} | {allocs:>9} |")
            
            output.append("")
    
    # Write to file
    with open('BENCHMARKS.md', 'w') as f:
        f.write('\n'.join(output))
    
    print("BENCHMARKS.md updated successfully")

if __name__ == '__main__':
    main()
