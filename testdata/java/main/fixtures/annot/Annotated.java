package fixtures.annot;

import java.io.IOException;
import java.util.List;

@Marker(id = 7, name = "annotated", type = String.class, level = Level.HIGH, ints = {4, 5, 6},
        tags = {"a", "b"}, inner = @Inner(x = 9, label = "deep"), big = 123456789012L, ratio = 0.75,
        flag = true, ch = 'Z', b = -1, s = 300, types = {Annotated.class}, levels = {Level.MID})
@BuildOnly("class-retention")
@SourceOnly
public class Annotated {
    @Marker(id = 1) @BuildOnly("field") public int field;
    @Inner(x = 2) public static final String CONST = "const";
    @Deprecated public String deprecatedField;

    @Marker(id = 5) public Annotated() {}

    @Marker(id = 2, tags = {"m"}) @BuildOnly("method")
    public int method(@Marker(id = 3) int p, @Inner(x = 4) String q, int unannotated) { return p + q.length() + unannotated; }

    @Deprecated public void deprecated() {}

    @SuppressWarnings("unchecked")                                   // SOURCE retention, absent
    public <T extends Comparable<T>> T max(List<T> xs) throws IOException {   // Signature + Throws
        if (xs.isEmpty()) throw new IOException("empty");
        T best = xs.get(0);
        for (T x : xs) if (x.compareTo(best) > 0) best = x;
        return best;
    }

    public static int annotatedParamsOnly(@Inner(x = 10) int a, @Inner(x = 11) long b, @Inner(x = 12) Object c) { return a + (int) b; }

    /** Nested annotation type. */
    public @interface NestedAnno { int v() default 0; }

    @NestedAnno(v = 3) public int nestedAnnotated() { return 3; }

    @Marker(id = 8) public static class AnnotatedNested {
        @Marker(id = 9) public int f;
    }
}
