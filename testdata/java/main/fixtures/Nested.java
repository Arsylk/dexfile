package fixtures;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Inner / anonymous / static nested classes, enum + ordinal switch, generics
 * (Signature annotations), native methods, varargs, bridge methods, local classes.
 */
public class Nested {
    private int outerValue = 5;
    int packageValue = 6;

    /** Non-static inner class (has synthetic this$0 field, InnerClass/EnclosingClass system annotations). */
    public class Inner {
        public int get() { return outerValue; }                     // nest-based private access
        public Nested outer() { return Nested.this; }
    }

    public static class StaticNested {
        public static int twice(int x) { return x * 2; }
        public int value = 1;
    }

    /** Anonymous class capturing a parameter (EnclosingMethod annotation). */
    public Runnable anonymous(final int k) {
        return new Runnable() {
            @Override public void run() { outerValue += k; }
        };
    }

    public Comparable<Nested> anonymousGeneric() {
        return new Comparable<Nested>() {
            @Override public int compareTo(Nested o) { return outerValue - o.outerValue; }   // bridge compareTo(Object)
        };
    }

    public static int localClass(int seed) {
        class Local {
            int v = seed;
            int doubled() { return v * 2; }
        }
        return new Local().doubled();
    }

    public enum Color {
        RED(1), GREEN(2), BLUE(3), ALPHA(0);
        public final int code;
        Color(int code) { this.code = code; }
        public static Color fromCode(int c) {
            for (Color col : values()) if (col.code == c) return col;
            return ALPHA;
        }
        public Color next() { return values()[(ordinal() + 1) % values().length]; }
    }

    public static int enumSwitch(Color c) {                      // ordinal -> packed-switch via $SwitchMap
        switch (c) {
            case RED: return 1;
            case GREEN: return 2;
            case BLUE: return 3;
            case ALPHA: return 4;
            default: return 0;
        }
    }

    public static String enumName(Color c) { return c.name().concat(String.valueOf(c.ordinal())); }
    public static Color enumValueOf(String s) { return Color.valueOf(s); }

    // ---- generics ----
    public interface Visitor<T> { T visit(Nested n); }
    public <T> T accept(Visitor<T> v) { return v.visit(this); }
    public static class StringVisitor implements Visitor<String> {
        @Override public String visit(Nested n) { return "visited"; }   // bridge: visit(Nested)Object
    }
    public static class Box<T extends Comparable<T>> {
        private T value;
        public T get() { return value; }
        public void set(T v) { value = v; }
        public boolean greater(Box<? extends T> other) { return value.compareTo(other.value) > 0; }
    }
    public static List<Map<String, Integer>> generic(List<? extends Number> in) {
        List<Map<String, Integer>> out = new ArrayList<>();
        for (Number n : in) {
            Map<String, Integer> m = new HashMap<>();
            m.put(n.toString(), n.intValue());
            out.add(m);
        }
        return out;
    }
    public static <K, V extends Comparable<V>> K maxKey(Map<K, V> m) {
        K best = null; V bestV = null;
        for (Map.Entry<K, V> e : m.entrySet()) {
            if (bestV == null || e.getValue().compareTo(bestV) > 0) { best = e.getKey(); bestV = e.getValue(); }
        }
        return best;
    }

    // ---- bridge via Comparable ----
    public static class Comparer implements Comparable<Comparer> {
        final int k;
        public Comparer(int k) { this.k = k; }
        @Override public int compareTo(Comparer o) { return Integer.compare(k, o.k); }   // + synthetic bridge compareTo(Object)
    }

    // ---- native ----
    public native int nativeMethod(int x);
    public static native void nativeStatic(String s, long l);
    public static native double nativeWide(double a, long b);

    // ---- varargs ----
    public static int varargs(int first, int... rest) { return first + rest.length; }
    public static int callVarargs() { return varargs(1, 2, 3, 4); }                   // filled-new-array
    public static int callVarargsEmpty() { return varargs(1); }                       // new-array 0
    public static String varargsObj(String fmt, Object... args) { return String.format(fmt, args); }
    public static String callVarargsObj(int a, String b) { return varargsObj("%d %s", a, b); }

    // ---- static init / instance init blocks ----
    static int staticInit;
    static { staticInit = 7; }
    int instanceInit;
    { instanceInit = 8; }

    public static int useAll(Nested n) {
        return n.new Inner().get() + StaticNested.twice(n.packageValue) + localClass(3) + enumSwitch(Color.GREEN)
                + n.accept(new StringVisitor()).length() + staticInit + n.instanceInit;
    }
}
