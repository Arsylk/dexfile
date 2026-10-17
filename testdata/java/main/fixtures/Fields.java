package fixtures;

/** iget/iput/sget/sput in every width variant, plus object and array fields. */
public class Fields {
    public int i;
    public long l;
    public float f;
    public double d;
    public boolean z;
    public byte b;
    public char c;
    public short s;
    public Object o;
    public String str;
    public int[] ia;
    public String[] sa;
    public long[][] lla;
    protected volatile int vol;
    transient int trans;

    public static int si;
    public static long sl;
    public static float sf;
    public static double sd;
    public static boolean sz;
    public static byte sb;
    public static char sc;
    public static short ss;
    public static Object so;
    public static String sstr;
    public static int[] sia;
    public static volatile long svol;

    public void putAll(int i, long l, float f, double d, boolean z, byte b, char c, short s, Object o) {
        this.i = i;         // iput
        this.l = l;         // iput-wide
        this.f = f;         // iput
        this.d = d;         // iput-wide
        this.z = z;         // iput-boolean
        this.b = b;         // iput-byte
        this.c = c;         // iput-char
        this.s = s;         // iput-short
        this.o = o;         // iput-object
        this.str = String.valueOf(o);
        this.ia = new int[]{i};
        this.sa = new String[]{str};
        this.lla = new long[1][1];
        this.vol = i;
        this.trans = i;
    }

    public double getAll() {
        int vi = i;         // iget
        long vl = l;        // iget-wide
        float vf = f;       // iget
        double vd = d;      // iget-wide
        boolean vz = z;     // iget-boolean
        byte vb = b;        // iget-byte
        char vc = c;        // iget-char
        short vs = s;       // iget-short
        Object vo = o;      // iget-object
        return vi + vl + vf + vd + (vz ? 1 : 0) + vb + vc + vs + (vo == null ? 0 : 1) + ia.length + sa.length + lla.length + vol + trans;
    }

    public static void sputAll(int i, long l, float f, double d, boolean z, byte b, char c, short s, Object o) {
        si = i;             // sput
        sl = l;             // sput-wide
        sf = f;             // sput
        sd = d;             // sput-wide
        sz = z;             // sput-boolean
        sb = b;             // sput-byte
        sc = c;             // sput-char
        ss = s;             // sput-short
        so = o;             // sput-object
        sstr = "static";
        sia = new int[4];
        svol = l;
    }

    public static double sgetAll() {
        int vi = si;        // sget
        long vl = sl;       // sget-wide
        float vf = sf;      // sget
        double vd = sd;     // sget-wide
        boolean vz = sz;    // sget-boolean
        byte vb = sb;       // sget-byte
        char vc = sc;       // sget-char
        short vs = ss;      // sget-short
        Object vo = so;     // sget-object
        return vi + vl + vf + vd + (vz ? 1 : 0) + vb + vc + vs + (vo == null ? 0 : 1) + sstr.length() + sia.length + svol;
    }

    public void increment() { i++; l++; c++; s++; b++; f += 1; d += 1; z = !z; }
    public static void sincrement() { si++; sl++; sc++; ss++; sb++; sf += 1; sd += 1; sz = !sz; }
    public int readOther(Fields other) { return other.i + (int) other.l; }
    public static int readSystem() { return java.lang.Integer.MAX_VALUE + (int) java.lang.Long.MIN_VALUE; }
    public static java.io.PrintStream out() { return System.out; }   // sget-object on external class
}
