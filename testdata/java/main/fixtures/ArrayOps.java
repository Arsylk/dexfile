package fixtures;

/**
 * new-array, array-length, fill-array-data (every primitive element type),
 * filled-new-array and filled-new-array/range, aget-* and aput-* for every element type,
 * multi-dimensional arrays.
 */
public class ArrayOps {
    static final int[] TABLE = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12};               // fill-array-data in <clinit>
    static final long[] LONG_TABLE = {1L, 2L, 3L, 4L, 5L, 6L, 7L, 8L, 9L, 10L};
    static final String[] NAMES = {"a", "b", "c", "d", "e", "f"};                      // filled-new-array/range in <clinit>

    public static byte[] bytes() { return new byte[]{1, 2, 3, 4, 5, 6, 7, 8, -1, 127, -128}; }
    public static short[] shorts() { return new short[]{100, 200, 300, 400, 500, 600, -700, 32767, -32768}; }
    public static char[] chars() { return new char[]{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'é', '￿'}; }
    public static int[] ints() { return new int[]{1, 2, 3, 5, 8, 13, 21, 34, 55, 89, -144}; }
    public static long[] longs() { return new long[]{1L, 2L, 3L, 4L, 5L, 6L, 7L, 8L, 0x0123456789ABCDEFL, -1L}; }
    public static float[] floats() { return new float[]{1f, 2f, 3f, 4f, 5f, 6f, 0.5f, -1.25f}; }
    public static double[] doubles() { return new double[]{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 0.125, -2.5}; }
    public static boolean[] bools() { return new boolean[]{true, false, true, true, false, false, true, false, true}; }

    public static int[] filledNewArray(int a, int b, int c) { return new int[]{a, b, c}; }          // filled-new-array
    public static int[] filledNewArray5(int a, int b, int c, int d, int e) { return new int[]{a, b, c, d, e}; }
    public static int[] filledNewArrayRange(int a, int b, int c, int d, int e, int f, int g) {
        return new int[]{a, b, c, d, e, f, g};                                                      // filled-new-array/range
    }
    public static Object[] filledNewArrayObj(Object a, Object b, Object c) { return new Object[]{a, b, c}; }
    public static String[] filledNewArrayRangeObj(String a, String b, String c, String d, String e, String f) {
        return new String[]{a, b, c, d, e, f};
    }

    public static int[] newArray(int n) { return new int[n]; }                                        // new-array
    public static Object[] newObjectArray(int n) { return new Object[n]; }
    public static int length(int[] a) { return a.length; }                                            // array-length
    public static int lengthObj(String[] a) { return a.length; }

    public static int[][] multi(int n, int m) { return new int[n][m]; }                              // multianewarray
    public static int[][][] multi3() { return new int[2][3][4]; }
    public static String[][] multiObj(int n) { return new String[n][2]; }
    public static int[][] jagged() { return new int[][]{{1, 2}, {3, 4, 5}, {}}; }
    public static int sumMulti(int[][] m) { int s = 0; for (int[] row : m) for (int v : row) s += v; return s; }

    // aput-* for every element type
    public static void putAll(int[] i, long[] l, double[] d, float[] f, Object[] o,
                              boolean[] z, byte[] b, char[] c, short[] s, int idx) {
        i[idx] = 1;             // aput
        l[idx] = 2L;            // aput-wide
        d[idx] = 3.0;           // aput-wide
        f[idx] = 4f;            // aput
        o[idx] = "five";        // aput-object
        z[idx] = true;          // aput-boolean
        b[idx] = 6;             // aput-byte
        c[idx] = 'g';           // aput-char
        s[idx] = 8;             // aput-short
    }

    // aget-* for every element type
    public static double getAll(int[] i, long[] l, double[] d, float[] f, Object[] o,
                                boolean[] z, byte[] b, char[] c, short[] s, int idx) {
        int vi = i[idx];        // aget
        long vl = l[idx];       // aget-wide
        double vd = d[idx];     // aget-wide
        float vf = f[idx];      // aget
        Object vo = o[idx];     // aget-object
        boolean vz = z[idx];    // aget-boolean
        byte vb = b[idx];       // aget-byte
        char vc = c[idx];       // aget-char
        short vs = s[idx];      // aget-short
        return vi + vl + vd + vf + (vo == null ? 0 : 1) + (vz ? 1 : 0) + vb + vc + vs;
    }

    public static int[] copyReverse(int[] in) {
        int[] out = new int[in.length];
        for (int k = 0; k < in.length; k++) out[in.length - 1 - k] = in[k];
        return out;
    }

    public static int[] clone(int[] in) { return in.clone(); }                                      // invoke-virtual [I.clone
    public static String[] stringArrayClone(String[] in) { return in.clone(); }
}
