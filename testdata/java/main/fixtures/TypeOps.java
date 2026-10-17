package fixtures;

/** instance-of, check-cast, new-instance, const-class, const-string with MUTF-8 edge cases. */
public class TypeOps {
    public static final String EMOJI = "emoji: 😀 🤔 🚀 done";   // non-BMP (surrogate pairs)
    public static final String NUL = "before\u0000after";                                   // embedded NUL -> MUTF-8 C0 80
    public static final String NON_ASCII = "héllo wörld — ñ 日本語 ру́сский العربية";
    public static final String LONG = "This is a deliberately long string constant used to exercise the MUTF-8 decoder with "
            + "more than two hundred and fifty-five bytes of content so that the ULEB128 length prefix needs two bytes. "
            + "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et "
            + "dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip.";
    public static final String EMPTY = "";
    public static final String MIXED = "a\u0000béc😀d\n\t\"quoted\" \\ end";

    public static boolean isString(Object o) { return o instanceof String; }                   // instance-of
    public static boolean isIntArray(Object o) { return o instanceof int[]; }
    public static boolean isObjArray(Object o) { return o instanceof Object[]; }
    public static boolean isCharSequence(Object o) { return o instanceof CharSequence; }         // interface
    public static boolean isTypeOps(Object o) { return o instanceof TypeOps; }

    public static String cast(Object o) { return (String) o; }                                 // check-cast
    public static Object[] castArray(Object o) { return (Object[]) o; }
    public static int[] castIntArray(Object o) { return (int[]) o; }
    public static Runnable castInterface(Object o) { return (Runnable) o; }
    public static Integer castBoxed(Object o) { return (Integer) o; }

    public static StringBuilder newInstance() { return new StringBuilder(16); }                 // new-instance
    public static TypeOps newSelf() { return new TypeOps(); }
    public static Object newObject() { return new Object(); }

    public static Class<?> constClass() { return TypeOps.class; }                                // const-class
    public static Class<?> constClassArray() { return String[].class; }
    public static Class<?> constClassIface() { return Runnable.class; }

    public static String emoji() { return EMOJI; }                                              // const-string
    public static String nul() { return NUL; }
    public static String nonAscii() { return NON_ASCII; }
    public static String longString() { return LONG; }
    public static String empty() { return EMPTY; }
    public static String mixed() { return MIXED; }
    public static int nulLength() { return "x\u0000y".length(); }

    public static String ternaryCast(Object o, boolean b) {
        CharSequence cs = b ? (CharSequence) o : "fallback";
        return cs.toString();
    }

    public static Object autoboxCast(Object o) {
        if (o instanceof Integer) return ((Integer) o) + 1;
        if (o instanceof Long) return ((Long) o) + 1L;
        if (o instanceof String) return ((String) o).length();
        return o;
    }
}
