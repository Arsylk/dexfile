package fixtures;

import java.lang.reflect.Field;
import java.lang.reflect.Method;

/** Reflection patterns: Class.forName, getDeclaredMethod, Method.invoke, getDeclaredField. */
public class Reflect {
    public static final String TARGET_NAME = "fixtures.Target";

    public static Object call() throws Exception {
        Class<?> c = Class.forName("fixtures.Target");
        Method m = c.getDeclaredMethod("secret");
        m.setAccessible(true);
        return m.invoke(c.getDeclaredConstructor().newInstance());
    }

    public static Object callWithArgs() throws Exception {
        Class<?> c = Class.forName(TARGET_NAME);
        Method m = c.getDeclaredMethod("add", int.class, int.class);     // filled-new-array of Class
        return m.invoke(null, 1, 2);                                     // Integer.valueOf boxing
    }

    public static Object fieldValue(Object target) throws Exception {
        Field f = Class.forName("fixtures.Target").getDeclaredField("hidden");
        f.setAccessible(true);
        return f.get(target);
    }

    public static Class<?> loadByLoader(String name) throws Exception {
        return Class.forName(name, true, Reflect.class.getClassLoader());
    }

    public static Object viaClassLiteral() throws Exception {
        return Target.class.getMethod("visible").invoke(new Target());
    }
}
