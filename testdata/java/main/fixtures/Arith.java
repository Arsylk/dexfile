package fixtures;

/**
 * Arithmetic, bitwise, shift, unary, conversion, comparison and if-* opcodes
 * in all their encodings (3-register, /2addr, /lit8, /lit16).
 */
public class Arith {
    // ---- int, 3-register form (dest differs from sources) ----
    public static int intAdd(int a, int b) { return a + b; }
    public static int intSub(int a, int b) { return a - b; }
    public static int intMul(int a, int b) { return a * b; }
    public static int intDiv(int a, int b) { return a / b; }
    public static int intRem(int a, int b) { return a % b; }
    public static int intAnd(int a, int b) { return a & b; }
    public static int intOr(int a, int b) { return a | b; }
    public static int intXor(int a, int b) { return a ^ b; }
    public static int intShl(int a, int b) { return a << b; }
    public static int intShr(int a, int b) { return a >> b; }
    public static int intUshr(int a, int b) { return a >>> b; }

    // ---- int /2addr form (accumulator) ----
    public static int int2addr(int a, int b) {
        int r = a;
        r += b; r -= b; r *= b; r /= b; r %= b;
        r &= b; r |= b; r ^= b;
        r <<= b; r >>= b; r >>>= b;
        return r;
    }

    // ---- int /lit8 ----
    public static int intLit8(int a) {
        int r = a + 5;      // add-int/lit8
        r = r - 3;          // add-int/lit8 -3
        r = r * 7;          // mul-int/lit8
        r = r / 3;          // div-int/lit8
        r = r % 5;          // rem-int/lit8
        r = r & 0x0f;       // and-int/lit8
        r = r | 0x10;       // or-int/lit8
        r = r ^ 0x55;       // xor-int/lit8
        r = r << 3;         // shl-int/lit8
        r = r >> 2;         // shr-int/lit8
        r = r >>> 1;        // ushr-int/lit8
        r = 5 - r;          // rsub-int/lit8
        return r;
    }

    // ---- int /lit16 ----
    public static int intLit16(int a) {
        int r = a + 300;    // add-int/lit16
        r = r * 300;        // mul-int/lit16
        r = r / 301;        // div-int/lit16
        r = r % 1000;       // rem-int/lit16
        r = r & 0x1234;     // and-int/lit16
        r = r | 0x1000;     // or-int/lit16
        r = r ^ 0x0fff;     // xor-int/lit16
        r = 300 - r;        // rsub-int
        return r;
    }

    // ---- long ----
    public static long longAdd(long a, long b) { return a + b; }
    public static long longSub(long a, long b) { return a - b; }
    public static long longMul(long a, long b) { return a * b; }
    public static long longDiv(long a, long b) { return a / b; }
    public static long longRem(long a, long b) { return a % b; }
    public static long longAnd(long a, long b) { return a & b; }
    public static long longOr(long a, long b) { return a | b; }
    public static long longXor(long a, long b) { return a ^ b; }
    public static long longShl(long a, int b) { return a << b; }
    public static long longShr(long a, int b) { return a >> b; }
    public static long longUshr(long a, int b) { return a >>> b; }
    public static long long2addr(long a, long b, int s) {
        long r = a;
        r += b; r -= b; r *= b; r /= b; r %= b;
        r &= b; r |= b; r ^= b;
        r <<= s; r >>= s; r >>>= s;
        return r;
    }

    // ---- float ----
    public static float floatAdd(float a, float b) { return a + b; }
    public static float floatSub(float a, float b) { return a - b; }
    public static float floatMul(float a, float b) { return a * b; }
    public static float floatDiv(float a, float b) { return a / b; }
    public static float floatRem(float a, float b) { return a % b; }
    public static float float2addr(float a, float b) {
        float r = a;
        r += b; r -= b; r *= b; r /= b; r %= b;
        return r;
    }

    // ---- double ----
    public static double doubleAdd(double a, double b) { return a + b; }
    public static double doubleSub(double a, double b) { return a - b; }
    public static double doubleMul(double a, double b) { return a * b; }
    public static double doubleDiv(double a, double b) { return a / b; }
    public static double doubleRem(double a, double b) { return a % b; }
    public static double double2addr(double a, double b) {
        double r = a;
        r += b; r -= b; r *= b; r /= b; r %= b;
        return r;
    }

    // ---- unary ----
    public static int negInt(int a) { return -a; }
    public static int notInt(int a) { return ~a; }
    public static long negLong(long a) { return -a; }
    public static long notLong(long a) { return ~a; }
    public static float negFloat(float a) { return -a; }
    public static double negDouble(double a) { return -a; }

    // ---- conversions ----
    public static long intToLong(int a) { return a; }
    public static float intToFloat(int a) { return a; }
    public static double intToDouble(int a) { return a; }
    public static int longToInt(long a) { return (int) a; }
    public static float longToFloat(long a) { return a; }
    public static double longToDouble(long a) { return a; }
    public static int floatToInt(float a) { return (int) a; }
    public static long floatToLong(float a) { return (long) a; }
    public static double floatToDouble(float a) { return a; }
    public static int doubleToInt(double a) { return (int) a; }
    public static long doubleToLong(double a) { return (long) a; }
    public static float doubleToFloat(double a) { return (float) a; }
    public static byte intToByte(int a) { return (byte) a; }
    public static char intToChar(int a) { return (char) a; }
    public static short intToShort(int a) { return (short) a; }

    public static int allConversions(int i, long l, float f, double d) {
        byte b = (byte) i;
        char c = (char) i;
        short s = (short) i;
        long l2 = i;
        float f2 = i;
        double d2 = i;
        int i3 = (int) l;
        float f3 = l;
        double d3 = l;
        int i4 = (int) f;
        long l4 = (long) f;
        double d4 = f;
        int i5 = (int) d;
        long l5 = (long) d;
        float f5 = (float) d;
        return b + c + s + (int) l2 + (int) f2 + (int) d2 + i3 + (int) f3 + (int) d3
                + i4 + (int) l4 + (int) d4 + i5 + (int) l5 + (int) f5;
    }

    // ---- comparisons ----
    public static int cmpLong(long a, long b) { return a < b ? -1 : (a > b ? 1 : 0); }  // cmp-long
    public static boolean cmplFloat(float a, float b) { return a > b; }                   // fcmpl -> cmpl-float
    public static boolean cmpgFloat(float a, float b) { return a < b; }                   // fcmpg -> cmpg-float
    public static boolean cmplDouble(double a, double b) { return a > b; }                // dcmpl -> cmpl-double
    public static boolean cmpgDouble(double a, double b) { return a < b; }                // dcmpg -> cmpg-double
    public static int cmpFloatFull(float a, float b) { if (a < b) return -1; if (a > b) return 1; return 0; }
    public static int cmpDoubleFull(double a, double b) { if (a <= b) return -1; if (a >= b) return 1; return 0; }

    // ---- if-* (two registers). javac inverts the condition, so ifEq emits if-ne etc. ----
    public static int ifEq(int a, int b) { if (a == b) return 1; return 0; }
    public static int ifNe(int a, int b) { if (a != b) return 1; return 0; }
    public static int ifLt(int a, int b) { if (a < b) return 1; return 0; }
    public static int ifGe(int a, int b) { if (a >= b) return 1; return 0; }
    public static int ifGt(int a, int b) { if (a > b) return 1; return 0; }
    public static int ifLe(int a, int b) { if (a <= b) return 1; return 0; }
    public static int ifEqLoop(int a, int b) { int n = 0; while (a == b) { a++; n++; } while (a != b) { a--; n++; } return n; }
    public static int ifLtLoop(int a, int b) { int n = 0; while (a < b) { a++; n++; } while (a >= b) { a--; n++; } return n; }
    public static int ifGtLoop(int a, int b) { int n = 0; while (a > b) { a--; n++; } while (a <= b) { a++; n++; } return n; }
    public static boolean ifRef(Object a, Object b) { return a == b; }

    // ---- if-*z (one register) ----
    public static int ifEqz(int a) { if (a == 0) return 1; return 0; }
    public static int ifNez(int a) { if (a != 0) return 1; return 0; }
    public static int ifLtz(int a) { if (a < 0) return 1; return 0; }
    public static int ifGez(int a) { if (a >= 0) return 1; return 0; }
    public static int ifGtz(int a) { if (a > 0) return 1; return 0; }
    public static int ifLez(int a) { if (a <= 0) return 1; return 0; }
    public static int ifzLoop(int a) { int n = 0; while (a > 0) { a--; n++; } while (a < 0) { a++; n++; } while (a != 0) { a = 0; } do { n++; } while (n <= 0); return n; }
    public static boolean ifNull(Object o) { return o == null; }
    public static boolean ifNonNull(Object o) { return o != null; }
    public static boolean ifBool(boolean z) { return !z; }
}
