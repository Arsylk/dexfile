package fixtures.multidex;

/** Lives in classes2.dex; called from Caller in classes.dex. */
public class Callee {
    public static final String NAME = "callee";
    public static int compute(int x) { return x * 2; }
    public int instanceCompute(int x) { return x + 100; }
}
