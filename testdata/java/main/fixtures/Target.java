package fixtures;

/** Reflection target for {@link Reflect}. */
public class Target {
    private int hidden = 3;
    public Target() {}
    private String secret() { return "s3cret"; }
    static int add(int a, int b) { return a + b; }
    public int visible() { return hidden; }
}
