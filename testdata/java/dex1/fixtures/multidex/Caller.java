package fixtures.multidex;

/** Lives in classes.dex; calls Callee which lives in classes2.dex. */
public class Caller {
    public static int call() { return Callee.compute(21); }
    public static String callField() { return Callee.NAME; }
    public static Callee make() { return new Callee(); }
    public static int callInstance(Callee c) { return c.instanceCompute(1); }
    public static String whichDup() { return Dup.which(); }
    public static int dupMagic() { return Dup.MAGIC; }
}
