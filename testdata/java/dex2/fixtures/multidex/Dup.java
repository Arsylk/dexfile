package fixtures.multidex;

/** Duplicate class: this body is in classes2.dex and differs from the classes.dex one. */
public class Dup {
    public static final int MAGIC = 2;
    public static String which() { return "classes2.dex"; }
    public int body() { return 2 * 2 * 2; }
    public static long extra() { return 0xdeadbeefL; }
}
