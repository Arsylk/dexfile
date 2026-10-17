package fixtures;

/**
 * Exercises every const* opcode form:
 * const/4, const/16, const, const/high16, const-wide/16, const-wide/32,
 * const-wide, const-wide/high16, const-string, const-class.
 */
public class Constants {
    public static int const4() { return 7; }                       // const/4
    public static int const4Neg() { return -8; }                   // const/4 -8
    public static int const16() { return 1000; }                   // const/16
    public static int const16Neg() { return -30000; }              // const/16
    public static int const32() { return 123456789; }              // const
    public static int const32Neg() { return -123456789; }          // const
    public static int constHigh16() { return 0x7f000000; }         // const/high16
    public static int constHigh16Neg() { return 0x80000000; }      // const/high16
    public static float floatOne() { return 1.0f; }                // const/high16 0x3f80
    public static float floatNeg() { return -2.5f; }               // const/high16 0xc020
    public static float floatPi() { return 3.14159f; }             // const
    public static long wide16() { return 42L; }                    // const-wide/16
    public static long wide16Neg() { return -42L; }                // const-wide/16
    public static long wide32() { return 100000L; }                // const-wide/32
    public static long wide32Neg() { return -100000L; }            // const-wide/32
    public static long wide64() { return 0x0123456789ABCDEFL; }    // const-wide
    public static long wideHigh16() { return 0x4000000000000000L; }// const-wide/high16
    public static double doubleOne() { return 1.0; }               // const-wide/high16 0x3ff0
    public static double doubleTwo() { return 2.0; }               // const-wide/high16 0x4000
    public static double doubleNeg() { return -1.0; }              // const-wide/high16 0xbff0
    public static double doublePi() { return 3.141592653589793; }  // const-wide
    public static char charConst() { return 'Z'; }
    public static char charHigh() { return '￿'; }
    public static short shortConst() { return (short) -1234; }
    public static byte byteConst() { return (byte) 0x7f; }
    public static boolean boolConst() { return true; }
    public static String str() { return "hello constants"; }       // const-string
    public static Class<?> cls() { return Constants.class; }       // const-class
    public static Class<?> primitiveCls() { return int.class; }    // sget-object Integer.TYPE
    public static Class<?> arrayCls() { return int[][].class; }    // const-class [[I
    public static Object nul() { return null; }                    // const/4 v0, 0

    public static int manyConsts(int x) {
        int a = 7;
        int b = 1000;
        int c = 0x7f000000;
        int d = 123456789;
        if (x > 0) { a = -8; b = -30000; c = 0x12340000; d = -1; }
        return x + a + b + c + d;
    }

    public static long manyWideConsts(int x) {
        long a = 42L;
        long b = 100000L;
        long c = 0x4000000000000000L;
        long d = 0x0123456789ABCDEFL;
        if (x > 0) { a = -42L; b = -100000L; c = -0x4000000000000000L; d = -2L; }
        return x + a + b + c + d;
    }
}
