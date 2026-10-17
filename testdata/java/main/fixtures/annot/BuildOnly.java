package fixtures.annot;

import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;

/** CLASS retention: present in the DEX (visibility BUILD) but not visible via reflection. */
@Retention(RetentionPolicy.CLASS)
public @interface BuildOnly {
    String value();
}
