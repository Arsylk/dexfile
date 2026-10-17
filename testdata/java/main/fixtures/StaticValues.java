package fixtures;

/**
 * static final fields with constant initializers: the class_def gets a
 * static_values_off pointing at an encoded_array (int, long, String, boolean,
 * double, float, char, short, byte, null).
 */
public class StaticValues {
    public static final int INT = 42;
    public static final int INT_NEG = -1;
    public static final int INT_BIG = 0x7fffffff;
    public static final long LONG = 1234567890123L;
    public static final long LONG_NEG = Long.MIN_VALUE;
    public static final String STRING = "static string value";
    public static final String STRING_UNICODE = "ünïcödé é 中文 🚀";
    public static final boolean BOOL_TRUE = true;
    public static final boolean BOOL_FALSE = false;
    public static final double DOUBLE = 2.718281828459045;
    public static final double DOUBLE_ONE = 1.0;
    public static final float FLOAT = 1.5f;
    public static final float FLOAT_NEG = -3.25f;
    public static final char CHAR = 'Q';
    public static final char CHAR_HIGH = '€';
    public static final short SHORT = -32000;
    public static final byte BYTE = -7;
    public static final Object NOTHING = null;
    public static final String NULL_STRING = null;
    public static final int[] NOT_CONSTANT = {1, 2};        // initialised in <clinit>
    public static final String COMPUTED = STRING.toUpperCase(); // initialised in <clinit>
    public static int MUTABLE = 5;                          // non-final with initial value (clinit sput)

    public static int sum() {
        return INT + (int) LONG + STRING.length() + (BOOL_TRUE ? 1 : 0) + (int) DOUBLE + (int) FLOAT + CHAR + SHORT + BYTE
                + (NOTHING == null ? 1 : 0) + NOT_CONSTANT.length + COMPUTED.length() + MUTABLE;
    }
}
