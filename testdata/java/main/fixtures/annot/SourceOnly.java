package fixtures.annot;

import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;

/** SOURCE retention: must NOT appear in the DEX. */
@Retention(RetentionPolicy.SOURCE)
public @interface SourceOnly {
}
