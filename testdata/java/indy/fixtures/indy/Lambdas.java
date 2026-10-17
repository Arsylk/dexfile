package fixtures.indy;

import java.util.List;
import java.util.function.BiFunction;
import java.util.function.Function;
import java.util.function.IntBinaryOperator;
import java.util.function.Supplier;

/** Lambdas and method references: invokedynamic -> invoke-custom (LambdaMetafactory) in the no-desugaring build. */
public class Lambdas {
    public final int base;
    public Lambdas() { this(0); }
    public Lambdas(int base) { this.base = base; }

    public static IntBinaryOperator nonCapturing() { return (a, b) -> a + b; }
    public static Runnable capturing(String s, int n) { return () -> System.out.println(s.length() + n); }
    public Function<Integer, Integer> capturingThis() { return x -> x + base; }
    public static Function<String, Integer> unboundRef() { return String::length; }
    public static Supplier<Lambdas> ctorRef() { return Lambdas::new; }
    public static Function<Integer, Lambdas> ctorRefArg() { return Lambdas::new; }
    public static BiFunction<String, Integer, Character> boundRef(String prefix) { return (s, i) -> prefix.concat(s).charAt(i); }
    public static Function<String, Boolean> boundRefMethod(String prefix) { return prefix::equalsIgnoreCase; }
    public static Function<Integer, int[]> arrayCtorRef() { return int[]::new; }
    public static Supplier<Long> staticRef() { return System::nanoTime; }

    public static int sum(List<Integer> xs) { return xs.stream().mapToInt(Integer::intValue).sum(); }
    public static int apply(IntBinaryOperator op, int a, int b) { return op.applyAsInt(a, b); }
    public static int useAll(String s) {
        return apply(nonCapturing(), 1, 2) + unboundRef().apply(s) + ctorRef().get().base
                + new Lambdas(5).capturingThis().apply(1) + arrayCtorRef().apply(3).length;
    }

    public interface Wide { long op(int a, long b, double c, int d, long e, double f); }
    public static Wide wideLambda(long k) { return (a, b, c, d, e, f) -> a + b + (long) c + d + e + (long) f + k; }
    public static long callWide(Wide w) { return w.op(1, 2L, 3.0, 4, 5L, 6.0); }
}
