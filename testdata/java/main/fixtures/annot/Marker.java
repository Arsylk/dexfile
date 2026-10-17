package fixtures.annot;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/** RUNTIME annotation with every element kind and default values (AnnotationDefault system annotation). */
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.TYPE, ElementType.FIELD, ElementType.METHOD, ElementType.PARAMETER, ElementType.CONSTRUCTOR})
public @interface Marker {
    int id() default 0;
    String name() default "unnamed";
    Class<?> type() default Object.class;
    Level level() default Level.LOW;
    int[] ints() default {1, 2, 3};
    String[] tags() default {};
    Inner inner() default @Inner(x = 1);
    long big() default 1L << 40;
    double ratio() default 0.5;
    float ratioF() default 0.25f;
    boolean flag() default false;
    char ch() default 'c';
    byte b() default 1;
    short s() default 2;
    Class<?>[] types() default {String.class, int.class};
    Level[] levels() default {Level.LOW, Level.HIGH};
}
