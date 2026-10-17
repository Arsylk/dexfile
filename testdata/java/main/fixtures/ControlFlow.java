package fixtures;

/** Loops, gotos, early returns, nested branches; produces goto, goto/16 and various if-* edges. */
public class ControlFlow {
    public static int loops(int n) {
        int acc = 0;
        for (int i = 0; i < n; i++) acc += i;
        int j = n;
        while (j > 0) { acc ^= j; j--; }
        do { acc++; } while (acc % 7 != 0);
        return acc;
    }

    public static int labeledBreak(int[][] m, int target) {
        int found = -1;
        outer:
        for (int i = 0; i < m.length; i++) {
            for (int j = 0; j < m[i].length; j++) {
                if (m[i][j] == target) { found = i * 1000 + j; break outer; }
                if (m[i][j] < 0) continue outer;
            }
        }
        return found;
    }

    public static int ternaryChain(int x) {
        return x < 0 ? -1 : x == 0 ? 0 : x < 10 ? 1 : x < 100 ? 2 : 3;
    }

    public static boolean shortCircuit(int a, int b, Object o) {
        return (a > 0 && b > 0) || (o != null && a != b) || (a == 0 && o == null);
    }

    public static int earlyReturns(int x) {
        if (x == 1) return 10;
        if (x == 2) return 20;
        if (x > 100) { if (x > 1000) return 1000; return 100; }
        return 0;
    }

    /** Long enough body in the loop that the back-edge needs goto/16 (offset > 127 units). */
    public static long goto16(int n) {
        long acc = 1;
        for (int i = 0; i < n; i++) {
            acc = acc * 31 + i;
            acc ^= (acc >>> 13);
            acc = acc * 17 + (i ^ 0x5bd1e995);
            acc += (acc << 7);
            acc ^= (acc >>> 11);
            acc = acc * 13 + (i * 3);
            acc ^= (acc << 5);
            acc = acc * 7 - (i ^ 0x1234567);
            acc += (acc >>> 3);
            acc ^= (acc << 17);
            acc = acc * 101 + (i ^ 0x7fed);
            acc -= (acc >>> 19);
            acc ^= (acc << 2);
            acc = acc * 3 + (i ^ 0xabcdef);
            acc += (acc >>> 29);
            acc ^= (acc << 9);
            acc = acc * 11 + (i * 1000003);
            acc -= (acc >>> 23);
            acc ^= (acc << 13);
            acc = acc * 41 + (i ^ 0x1f1f1f);
        }
        return acc;
    }

    public static int infiniteLoopWithBreak(int x) {
        int n = 0;
        while (true) {
            n++;
            if (n > x) break;
        }
        return n;
    }

    public static void emptyLoop(int n) { for (int i = 0; i < n; i++) { } }
    public static void noop() { }
    public static int unreachableAfterThrow(int x) { if (x > 0) { throw new RuntimeException(); } return x; }
}
