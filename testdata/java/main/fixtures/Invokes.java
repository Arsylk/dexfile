package fixtures;

/**
 * Every invoke kind: invoke-virtual, invoke-super, invoke-direct, invoke-static,
 * invoke-interface, and their /range variants (6+ parameters incl. wide params).
 */
public class Invokes {
    public static class Base {
        protected Base() {}
        public int value() { return 1; }
        public String name() { return "base"; }
        public long wide(int a, long b, double c, int d, long e, double f) { return a + b + (long) c + d + e + (long) f; }
    }

    public static class Child extends Base {
        public Child() { super(); }                                                 // invoke-direct <init>
        @Override public int value() { return super.value() + 1; }                  // invoke-super
        @Override public String name() { return super.name().concat("/child"); }    // invoke-super + invoke-virtual
        @Override public long wide(int a, long b, double c, int d, long e, double f) {
            return super.wide(a, b, c, d, e, f) * 2;                                // invoke-super/range
        }
        private int secret() { return 42; }
        public int callSecret() { return secret(); }                                // invoke-direct (private)
        private long secretWide(int a, long b, double c, int d, long e, double f) { return a + b + (long) c + d + e + (long) f; }
        public long callSecretWide() { return secretWide(1, 2L, 3.0, 4, 5L, 6.0); } // invoke-direct/range
    }

    public static class Ctor6 {
        final long sum;
        public Ctor6(int a, long b, double c, int d, long e, double f) { sum = a + b + (long) c + d + e + (long) f; }
    }

    public interface Greeter {
        String greet(String who);
        long wideGreet(int a, long b, double c, int d, long e, double f);
        default String hello() { return greet("default"); }                          // default method
        static Greeter none() { return null; }                                       // static interface method
    }

    public static class HelloGreeter implements Greeter {
        public String greet(String who) { return "hi ".concat(who); }
        public long wideGreet(int a, long b, double c, int d, long e, double f) { return a - b; }
    }

    public static String viaInterface(Greeter g) { return g.greet("x"); }                        // invoke-interface
    public static String viaDefault(Greeter g) { return g.hello(); }
    public static int viaVirtual(Base b) { return b.value(); }                                   // invoke-virtual
    public static int viaStatic(int x) { return helper(x); }                                     // invoke-static
    static int helper(int x) { return x * 2; }

    public static long staticRange(int a, long b, double c, int d, float e, Object f) {
        return a + b + (long) c + d + (long) e + (f == null ? 0 : 1);
    }
    public static long callStaticRange() { return staticRange(1, 2L, 3.0, 4, 5f, "six"); }     // invoke-static/range
    public static long callVirtualRange(Base b) { return b.wide(1, 2L, 3.0, 4, 5L, 6.0); }      // invoke-virtual/range
    public static long callInterfaceRange(Greeter g) { return g.wideGreet(1, 2L, 3.0, 4, 5L, 6.0); } // invoke-interface/range
    public static Ctor6 newCtor6() { return new Ctor6(1, 2L, 3.0, 4, 5L, 6.0); }               // invoke-direct/range <init>

    public static Object chain(String s) { return new StringBuilder(s).append(1).append('c').append(2L).append(3.0).toString(); }
    public static int viaObject(Object o) { return o.hashCode() + o.toString().length(); }   // invoke-virtual on Object
    public static boolean equalsCall(Object a, Object b) { return a.equals(b); }
    public static int staticExternal(String s) { return Integer.parseInt(s) + Math.abs(-1); }
    public static Object[] boxing(int i, long l, double d, boolean z, char c) { return new Object[]{i, l, d, z, c}; } // Integer.valueOf etc.
    public static int unboxing(Integer i, Long l) { return i + (int) (long) l; }               // intValue/longValue
}
