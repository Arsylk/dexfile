#!/usr/bin/env python3
"""Generate the sources for testdata/dex/container041.dex.

740 interfaces with 90 abstract methods each reference 66,600 distinct
method_ids, more than the 65,535 one DEX section can hold, so d8 in container
mode emits a two-section DEX 041 container. Main is the only class with code.
The counts are asserted by TestContainer041; change them together.
"""
import pathlib
import sys

N, M = 740, 90

out = pathlib.Path(sys.argv[1]) / "c"
out.mkdir(parents=True, exist_ok=True)
for i in range(N):
    body = "\n".join(f"  int m{k}(int x);" for k in range(M))
    (out / f"I{i}.java").write_text(f"package c;\ninterface I{i} {{\n{body}\n}}\n")
(out / "Main.java").write_text(
    "package c;\npublic final class Main implements I0 {\n"
    + "\n".join(f"  public int m{k}(int x) {{ return x * {k} + 1; }}" for k in range(M))
    + "\n  public static int run(I0 i, int x) { int s = 0; for (int k = 0; k < x; k++) { s += i.m0(k); } return s; }\n"
    + "  public static void main(String[] args) { System.out.println(run(new Main(), args.length)); }\n}\n")
