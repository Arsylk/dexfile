package fixtures.indy;

/** String concatenation: javac 9+ emits invokedynamic StringConcatFactory.makeConcatWithConstants. */
public class Concat {
    public static String concat(String a, int b, long c, Object d) { return "a=" + a + " b=" + b + " c=" + c + " d=" + d; }
    public static String concat2(char c, double d, boolean z, float f) { return c + ":" + d + ":" + z + ":" + f; }
    public static String concatConst(int x) { return "value é😀 " + x + '!' ; }
    public static String concatMany(int a, int b, int c, int d, int e, int f, int g, int h) {
        return "" + a + b + c + d + e + f + g + h;
    }
    public static String loopConcat(String[] parts) {
        String s = "";
        for (String p : parts) s += p + ",";
        return s;
    }
}
