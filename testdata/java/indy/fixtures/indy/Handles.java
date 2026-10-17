package fixtures.indy;

import java.lang.invoke.MethodHandle;
import java.lang.invoke.MethodHandles;
import java.lang.invoke.MethodType;

/** java.lang.invoke usage: MethodHandle.invokeExact / invoke -> invoke-polymorphic (DEX 038, API 26+). */
public class Handles {
    public int instanceField = 3;
    public static int staticField = 4;

    public static int target(int a, int b) { return a + b; }
    public String vtarget(String s) { return s.toUpperCase(); }
    public static long wideTarget(int a, long b, double c, int d, long e, double f) { return a + b + (long) c + d + e + (long) f; }

    public static int callExact() throws Throwable {
        MethodHandle mh = MethodHandles.lookup().findStatic(Handles.class, "target",
                MethodType.methodType(int.class, int.class, int.class));
        return (int) mh.invokeExact(1, 2);                          // invoke-polymorphic, proto (II)I
    }

    public static Object callInvoke(Object x) throws Throwable {
        MethodHandle mh = MethodHandles.lookup().findVirtual(Handles.class, "vtarget",
                MethodType.methodType(String.class, String.class));
        return mh.invoke(new Handles(), x);                          // invoke-polymorphic, proto (Handles,Object)Object
    }

    public static long callExactRange() throws Throwable {
        MethodHandle mh = MethodHandles.lookup().findStatic(Handles.class, "wideTarget",
                MethodType.methodType(long.class, int.class, long.class, double.class, int.class, long.class, double.class));
        return (long) mh.invokeExact(1, 2L, 3.0, 4, 5L, 6.0);       // invoke-polymorphic/range
    }

    public static void callVoid(MethodHandle mh, Object arg) throws Throwable {
        mh.invokeExact(arg);                                         // invoke-polymorphic, proto (Object)V
    }

    public static int fieldHandles() throws Throwable {
        MethodHandles.Lookup lookup = MethodHandles.lookup();
        MethodHandle getter = lookup.findGetter(Handles.class, "instanceField", int.class);
        MethodHandle sgetter = lookup.findStaticGetter(Handles.class, "staticField", int.class);
        MethodHandle ssetter = lookup.findStaticSetter(Handles.class, "staticField", int.class);
        ssetter.invokeExact(10);
        return (int) getter.invokeExact(new Handles()) + (int) sgetter.invokeExact();
    }

    public static MethodType type() { return MethodType.methodType(void.class, int.class, String.class); }
}
