package fixtures.shapes;

/** A class whose method is not overridden anywhere (single dispatch target). */
public class Unique {
    public int notOverridden() { return 99; }
    public static int callUnique(Unique u) { return u.notOverridden(); }
}
