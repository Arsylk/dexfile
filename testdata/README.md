# Test fixtures

Binary DEX fixtures for the parser/analysis tests, plus the Java sources and the
script that build them. Everything under `dex/` is generated; rebuild with:

```sh
./testdata/build.sh          # needs javac >= 11, python3, Android build-tools 37.0.0 (d8, dexdump)
```

Environment overrides: `ANDROID_SDK_ROOT`, `BUILD_TOOLS`, `ANDROID_JAR`, `JAVAC`, `PYTHON`.
The script is idempotent (wipes `testdata/build` and `testdata/dex` first). It takes about
30 s. The `MethodHandle.invoke ... only supported starting with Android O` warnings it prints
for the `--min-api 21/24` builds are expected (see caveats).

## Layout

| Path | Contents |
| --- | --- |
| `java/main/fixtures/**` | Main fixture set (package `fixtures` and sub-packages `shapes`, `annot`, `large`) |
| `java/main/fixtures/large/` | **Generated** by `scripts/gen_large.py` (do not edit): `Large.java` (~470 KB), `ManyMethods.java` |
| `java/indy/fixtures/indy/` | invokedynamic / MethodHandle sources (`Lambdas`, `Concat`, `Handles`) |
| `java/tiny/fixtures/tiny/Hello.java` | Single trivial class for `tiny.dex` |
| `java/dex1/fixtures/multidex/` | `Caller` (calls across dex files) and `Dup` (body #1) for `classes.dex` |
| `java/dex2/fixtures/multidex/` | `Callee` and `Dup` (body #2) for `classes2.dex` |
| `build.sh` | Build script |
| `dex/` | Outputs: `*.dex`, `multidex.apk`, `malformed/`, and `dexdump` oracles |

All sources are compiled with `javac --release 11 -g -parameters -encoding UTF-8`, so every
debug build carries line numbers, local variable names and `MethodParameters` annotations.

## Fixtures

| File | Magic | d8 flags | Inputs | Size |
| --- | --- | --- | --- | --- |
| `dex/v035.dex` | `dex\n035` | `--min-api 21 --debug` | main + indy | 285,876 B |
| `dex/v037.dex` | `dex\n037` | `--min-api 24 --debug` | main + indy | 285,280 B |
| `dex/v038.dex` | `dex\n038` | `--min-api 26 --debug --no-desugaring` | main + indy | 282,324 B |
| `dex/v039.dex` | `dex\n039` | `--min-api 28 --debug` | main + indy | 285,200 B |
| `dex/release.dex` | `dex\n035` | `--min-api 21 --release` (no debug info) | main + indy | 215,004 B |
| `dex/tiny.dex` | `dex\n035` | `--min-api 21 --debug` | tiny | 1,072 B |
| `dex/multidex.apk` | zip | `classes.dex`: `--min-api 21 --debug --classpath classes-dex2` (main + dex1); `classes2.dex`: `--min-api 21 --debug` (dex2) | | 278,319 B |
| `dex/malformed/*.dex` | n/a | hand-corrupted copies of `tiny.dex` (see below) | | |

Every `--lib` is `platforms/android-36/android.jar`. All inputs are the same class files; only
the d8 flags differ, so the `v035`/`v037`/`v038`/`v039`/`release` files are directly
comparable for differential tests.

Beside each DEX: `<name>.dexdump.txt` (`dexdump -d`) and `<name>.dexdump-f.txt` (`dexdump -f`).
For the APK the two members are dumped as `multidex.classes.dexdump*.txt` and
`multidex.classes2.dexdump*.txt`.

### Header counts (`dexdump -f`)

| File | string_ids | type_ids | proto_ids | field_ids | method_ids | class_defs | file_size |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `v035.dex` | 1693 | 160 | 217 | 119 | 920 | 64 | 285876 |
| `v037.dex` | 1690 | 159 | 217 | 119 | 918 | 63 | 285280 |
| `v038.dex` | 1660 | 148 | 227 | 113 | 888 | 49 | 282324 |
| `v039.dex` | 1689 | 158 | 219 | 119 | 919 | 62 | 285200 |
| `release.dex` | 1284 | 159 | 217 | 119 | 917 | 63 | 215004 |
| `tiny.dex` | 19 | 8 | 3 | 1 | 4 | 1 | 1072 |
| `multidex.apk!classes.dex` | 1582 | 133 | 185 | 109 | 842 | 48 | 276712 |
| `multidex.apk!classes2.dex` | 26 | 8 | 5 | 2 | 8 | 2 | 1412 |

Map-list item types present in every file: header, string_id, type_id, proto_id, field_id,
method_id, class_def, map_list, type_list, annotation_set_ref_list, annotation_set,
class_data, code, string_data, debug_info, annotation, encoded_array, annotations_directory.
`v038.dex` additionally has `call_site_id` (17 entries) and `method_handle` (14 entries).
`release.dex` still has 398 `debug_info` items: `d8 --release` drops local-variable records
(only ~250 parameter entries remain, vs ~11,400 in `v035.dex`) but keeps a reduced line-number
table (641 `line=` positions vs 11,431), so stack traces stay meaningful.

### multidex.apk

Zip entries: `AndroidManifest.xml` (64 placeholder bytes, not valid binary XML),
`classes.dex` (stored), `classes2.dex` (deflated), `resources.arsc` (32 placeholder bytes),
`META-INF/junk.txt`, `assets/notes.txt`.

* Cross-dex call: `fixtures.multidex.Caller` (classes.dex) invokes `fixtures.multidex.Callee.compute(int)`,
  reads `Callee.NAME`, constructs `Callee`, and calls `Callee.instanceCompute` -- all defined only in classes2.dex.
* Duplicate class: `fixtures.multidex.Dup` is defined in **both** dex files with different bodies:
  classes.dex `MAGIC = 1`, `which()` returns `"classes.dex"`, `body()` is `1 + 1`;
  classes2.dex `MAGIC = 2`, `which()` returns `"classes2.dex"`, `body()` is `2 * 2 * 2`, plus an extra `extra()` method.

### malformed/

All derived from `tiny.dex` (1072 bytes) by `build.sh`:

| File | Corruption |
| --- | --- |
| `empty.dex` | 0 bytes |
| `truncated-header.dex` | first 60 bytes (header is 112) |
| `truncated-mid.dex` | first 536 bytes (half the file) |
| `bad-magic.dex` | magic replaced with `xex\n035\0` |
| `huge-sizes.dex` | `string_ids_size` (offset 0x38) = `0xFFFFFFFF` |
| `bad-offsets.dex` | `map_off` (0x34) = `0xFFFFFFF0`, `class_defs_off` (0x64) = `0x7FFFFFFF` |
| `wrong-file-size.dex` | header `file_size` (0x20) = 16x the real size |
| `bad-endian-tag.dex` | `endian_tag` (0x28) = `0x78563412` (REVERSE_ENDIAN_CONSTANT) |

## Feature map

Method names below are from `fixtures.*` in the main set unless stated otherwise. Counts are
occurrences in `v038.dexdump.txt` (the superset build); `v035`/`v037`/`v039` are near-identical
except where noted, and `release` differs in register allocation (more `/2addr`, fewer `move`s).

### Switch payloads and control flow
| Feature | Where |
| --- | --- |
| packed-switch | `Switches.packed(int)` (10 dense cases), `Switches.packedFallthrough`, `Switches.charSwitch`, `Switches.nestedSwitch`, `Switches.switchInLoop`, `Nested.enumSwitch(Nested$Color)` (ordinal switch via `Nested$1.$SwitchMap$...`) |
| sparse-switch | `Switches.sparse(int)` (cases -5, 1, 100, 10000, 1000000, MAX_VALUE), `Switches.sparseNoDefault`, `Switches.stringSwitch(String)` (hashCode switch, incl. colliding `"Aa"`/`"BB"`) |
| goto | everywhere; `ControlFlow.loops`, `ControlFlow.labeledBreak`, `ControlFlow.infiniteLoopWithBreak` |
| goto/16 | `large.Large.hugeLocals(int)` (also `ControlFlow.goto16` has a loop body ~100 units; d8 encoded it as plain `goto`) |
| goto/32 | `large.Large.loopLinear(int,boolean)` (loop body ~45k code units; `release.dex` has none because of layout differences) |
| if-eq/ne/lt/ge/gt/le | `Arith.ifEq..ifLe`, `Arith.ifEqLoop/ifLtLoop/ifGtLoop`, `Arith.ifRef` (javac inverts: `ifEq` emits `if-ne`, etc.; the loop variants give the other polarity) |
| if-eqz/nez/ltz/gez/gtz/lez | `Arith.ifEqz..ifLez`, `Arith.ifzLoop`, `Arith.ifNull/ifNonNull/ifBool` |
| ternaries / short-circuit | `ControlFlow.ternaryChain`, `ControlFlow.shortCircuit`, `ControlFlow.earlyReturns` |
| nop (payload alignment) | `ArrayOps.<clinit>`, `Exceptions.finallyBlock`, `Exceptions.nestedFinally` |

### Constants
| Opcode | Where |
| --- | --- |
| const/4 | `Constants.const4`, `Constants.const4Neg`, `Constants.nul` |
| const/16 | `Constants.const16`, `Constants.const16Neg`, `Constants.shortConst` |
| const | `Constants.const32`, `Constants.const32Neg`, `Constants.floatPi`, `Constants.manyConsts` |
| const/high16 | `Constants.constHigh16` (0x7f000000), `Constants.constHigh16Neg` (0x80000000), `Constants.floatOne` (1.0f), `Constants.floatNeg`, `Constants.manyConsts` |
| const-wide/16 | `Constants.wide16`, `Constants.wide16Neg`, `Constants.manyWideConsts` |
| const-wide/32 | `Constants.wide32`, `Constants.wide32Neg`, `Constants.manyWideConsts` |
| const-wide | `Constants.wide64` (0x0123456789ABCDEF), `Constants.doublePi`, `shapes.Circle.area` (Math.PI) |
| const-wide/high16 | `Constants.wideHigh16`, `Constants.doubleOne` (1.0), `Constants.doubleTwo` (2.0), `Constants.doubleNeg`, `Constants.manyWideConsts` |
| const-string | `Constants.str`, `Switches.packed`, `TypeOps.*` |
| const-class | `Constants.cls`, `Constants.arrayCls` (`[[I`), `TypeOps.constClass*`, `Exceptions.synchronizedStaticBlock` |
| MUTF-8 edge cases | `TypeOps.EMOJI` / `TypeOps.emoji()` (non-BMP, surrogate pairs encoded as 3+3 bytes), `TypeOps.NUL` (`\u0000` -> `C0 80`), `TypeOps.NON_ASCII`, `TypeOps.LONG` (utf16 length > 255 so the ULEB128 size prefix is 2 bytes), `TypeOps.EMPTY`, `TypeOps.MIXED`, `StaticValues.STRING_UNICODE` |

### Arithmetic / bitwise / conversions / compares
| Opcode group | Where |
| --- | --- |
| add/sub/mul/div/rem/and/or/xor/shl/shr/ushr-int (3-reg) | `Arith.intAdd` ... `Arith.intUshr` |
| *-int/2addr | `Arith.int2addr` |
| add/mul/div/rem/and/or/xor/shl/shr/ushr-int/lit8, rsub-int/lit8 | `Arith.intLit8` (`5 - r` is `rsub-int/lit8`) |
| add/mul/div/rem/and/or/xor-int/lit16, rsub-int | `Arith.intLit16` (`300 - r` is `rsub-int`) |
| *-long (3-reg), *-long/2addr | `Arith.longAdd` ... `Arith.longUshr`, `Arith.long2addr` (incl. `ushr-long/2addr`) |
| *-float, *-float/2addr | `Arith.floatAdd` ... `Arith.floatRem`, `Arith.float2addr` |
| *-double, *-double/2addr | `Arith.doubleAdd` ... `Arith.doubleRem`, `Arith.double2addr` |
| neg-int, not-int, neg-long, not-long, neg-float, neg-double | `Arith.negInt`, `Arith.notInt`, `Arith.negLong`, `Arith.notLong`, `Arith.negFloat`, `Arith.negDouble` |
| int-to-long/float/double, long-to-int/float/double, float-to-int/long/double, double-to-int/long/float | `Arith.intToLong` ... `Arith.doubleToFloat`, all together in `Arith.allConversions` |
| int-to-byte, int-to-char, int-to-short | `Arith.intToByte/intToChar/intToShort`, `Arith.allConversions`, `Fields.increment` |
| cmp-long | `Arith.cmpLong` |
| cmpl-float / cmpg-float | `Arith.cmplFloat` (`a > b`) / `Arith.cmpgFloat` (`a < b`), both in `Arith.cmpFloatFull` |
| cmpl-double / cmpg-double | `Arith.cmplDouble` / `Arith.cmpgDouble`, both in `Arith.cmpDoubleFull` |

### Invokes
| Opcode | Where |
| --- | --- |
| invoke-virtual | `Invokes.viaVirtual`, `shapes.ShapeUser.viaAbstract`, almost everywhere |
| invoke-super | `Invokes$Child.value()`, `Invokes$Child.name()` |
| invoke-direct | every `<init>`; private: `Invokes$Child.callSecret()` -> `secret()`, `large.ManyMethods.callPrivate` |
| invoke-static | `Invokes.viaStatic`, `Reflect.*` |
| invoke-interface | `Invokes.viaInterface` (`Invokes$Greeter.greet`), `Invokes.viaDefault` (default method), `shapes.ShapeUser.viaInterface` |
| invoke-virtual/range | `Invokes.callVirtualRange` (`Base.wide(IJDIJD)J`), `large.Large.manyLocals` |
| invoke-super/range | `Invokes$Child.wide(IJDIJD)J` |
| invoke-direct/range | `Invokes.newCtor6` (`Ctor6.<init>(IJDIJD)`), `Invokes$Child.callSecretWide` (private) |
| invoke-static/range | `Invokes.callStaticRange` (`staticRange(IJDIFLjava/lang/Object;)J`), `large.Large.manyLocals` |
| invoke-interface/range | `Invokes.callInterfaceRange` (`Greeter.wideGreet`), `indy.Lambdas.callWide` |
| invoke-custom (038 only) | `indy.Concat.concat/concat2/concatConst/loopConcat` (StringConcatFactory), `indy.Lambdas.*` (LambdaMetafactory: non-capturing, capturing, `this`-capturing, unbound/bound/static/constructor/array-constructor method refs) |
| invoke-custom/range (038 only) | `indy.Concat.concatMany(IIIIIIII)` |
| invoke-polymorphic (038 only) | `indy.Handles.callExact` (`invokeExact`, proto `(II)I`), `indy.Handles.callInvoke` (`invoke`), `indy.Handles.callVoid`, `indy.Handles.fieldHandles` (getter/setter handles) |
| invoke-polymorphic/range (038 only) | `indy.Handles.callExactRange` (proto `(IJDIJD)J`) |
| boxing / unboxing | `Invokes.boxing`, `Invokes.unboxing`, `Reflect.callWithArgs` |
| varargs call site | `Nested.callVarargs` (filled-new-array), `Nested.callVarargsEmpty` (zero-length new-array), `Nested.callVarargsObj` |

### Fields and static values
| Feature | Where |
| --- | --- |
| iput / iput-wide / iput-object / iput-boolean / iput-byte / iput-char / iput-short | `Fields.putAll`, `Fields.increment` |
| iget-* (all variants) | `Fields.getAll`, `Fields.readOther` |
| sput-* (all variants) | `Fields.sputAll`, `Fields.sincrement` |
| sget-* (all variants) | `Fields.sgetAll`, `Fields.readSystem`, `Fields.out()` (`System.out`) |
| volatile / transient fields | `Fields.vol`, `Fields.svol`, `Fields.trans` |
| `static_values_off` encoded_array | class `StaticValues`: int (`INT`, `INT_NEG`, `INT_BIG`), long (`LONG`, `LONG_NEG` = MIN_VALUE), String (`STRING`, `STRING_UNICODE`), boolean (`BOOL_TRUE`, `BOOL_FALSE`), double (`DOUBLE`, `DOUBLE_ONE`), float (`FLOAT`, `FLOAT_NEG`), char (`CHAR`, `CHAR_HIGH`), short (`SHORT`), byte (`BYTE`), null (`NOTHING`, `NULL_STRING` -- encoded as VALUE_NULL because `STRING` sorts after them). `NOT_CONSTANT`, `COMPUTED`, `MUTABLE` are initialised in `<clinit>` instead. Also `ArrayOps.TABLE/LONG_TABLE/NAMES` (clinit), enum `$VALUES` |

### Arrays
| Opcode | Where |
| --- | --- |
| fill-array-data | `ArrayOps.bytes()` `[B`, `shorts()` `[S`, `chars()` `[C`, `ints()` `[I`, `longs()` `[J`, `floats()` `[F`, `doubles()` `[D`, `bools()` `[Z`, and `ArrayOps.<clinit>` (`TABLE`, `LONG_TABLE`) |
| filled-new-array | `ArrayOps.filledNewArray(III)`, `ArrayOps.filledNewArray5`, `ArrayOps.filledNewArrayObj`, `ArrayOps.jagged`, `Fields.putAll`, `Reflect.callWithArgs` (Class[]) |
| filled-new-array/range | `ArrayOps.filledNewArrayRange(IIIIIII)[I`, `ArrayOps.filledNewArrayRangeObj` (6 Strings), `ArrayOps.<clinit>` (`NAMES`) |
| new-array | `ArrayOps.newArray`, `ArrayOps.newObjectArray`, `Nested.callVarargsEmpty` |
| array-length | `ArrayOps.length`, `ArrayOps.lengthObj`, `ArrayOps.copyReverse` |
| aget / aget-wide / aget-object / aget-boolean / aget-byte / aget-char / aget-short | `ArrayOps.getAll` |
| aput / aput-wide / aput-object / aput-boolean / aput-byte / aput-char / aput-short | `ArrayOps.putAll` |
| multi-dimensional | `ArrayOps.multi(II)[[I`, `ArrayOps.multi3()[[[I`, `ArrayOps.multiObj`, `ArrayOps.jagged`, `ArrayOps.sumMulti`, `Fields.lla` (`[[J`) |
| array clone | `ArrayOps.clone`, `ArrayOps.stringArrayClone` |

### Exceptions and monitors
| Feature | Where |
| --- | --- |
| multiple typed handlers | `Exceptions.multiCatch` (NumberFormatException, NullPointerException, RuntimeException), `Exceptions.multiCatchUnion` (multi-catch `A \| B`) |
| catch-all (`Throwable`) | `Exceptions.catchAll` |
| finally | `Exceptions.finallyBlock`, `Exceptions.tryCatchFinally`, `Exceptions.nestedFinally`, `Exceptions.finallyWithReturnInTry` |
| nested try | `Exceptions.nested`, `Exceptions.nestedFinally` |
| handler re-throws | `Exceptions.rethrow` (wraps), `Exceptions.rethrowSame` (`throw e`) |
| explicit throw | `Exceptions.throwsExplicit` (IllegalStateException), `Exceptions.throwsChecked` (IOException), `ControlFlow.unreachableAfterThrow` |
| handler in loop / continue from handler | `Exceptions.catchInLoop` |
| move-exception | every handler above (23 in v038) |
| monitor-enter / monitor-exit | `Exceptions.synchronizedBlock` (on a field), `Exceptions.synchronizedBlockWithThrow` (on `this`, with throw inside), `Exceptions.synchronizedStaticBlock` (on a class literal); d8 also emits them for `Exceptions.staticSynchronized` |
| ACC_SYNCHRONIZED | `Exceptions.synchronizedMethod`, `Exceptions.staticSynchronized` |

### Types, classes, hierarchy
| Feature | Where |
| --- | --- |
| instance-of | `TypeOps.isString`, `isIntArray` (`[I`), `isObjArray`, `isCharSequence` (interface), `isTypeOps`, `autoboxCast` |
| check-cast | `TypeOps.cast`, `castArray`, `castIntArray`, `castInterface`, `castBoxed`, `ternaryCast` |
| new-instance | `TypeOps.newInstance`, `newSelf`, `newObject`; `Invokes.newCtor6` |
| interface dispatch | `shapes.Shape.area()` called by `shapes.ShapeUser.viaInterface`, `ShapeUser.all` |
| abstract/virtual dispatch | `shapes.BaseShape.area()` (abstract) called by `ShapeUser.viaAbstract`, `BaseShape.scaled`; `Square.area` by `ShapeUser.viaSquare` (targets `Square` and `Rect`) |
| override chain | `BaseShape` (abstract) <- `Circle`, `Square` <- `Rect` (overrides `area`, inherits `describe`, `corners`) |
| inherited, not overridden | `ShapeUser.inheritedDescribe(Rect)` and `ShapeUser.inheritedCorners(Rect)` resolve to `Square.describe/corners` |
| never-overridden method | `shapes.Unique.notOverridden()`, called by `Unique.callUnique` |
| inner class (this$0, InnerClass/EnclosingClass) | `Nested$Inner` |
| static nested | `Nested$StaticNested`, `Invokes$Base/$Child/$Ctor6/$HelloGreeter`, `annot.Annotated$AnnotatedNested` |
| anonymous (EnclosingMethod) | `Nested$1` (Runnable, in `Nested.anonymous`), `Nested$2` (Comparable, in `Nested.anonymousGeneric`) |
| local class | `Nested$1Local` (in `Nested.localClass`) |
| enum | `Nested$Color` (RED, GREEN, BLUE, ALPHA with field `code`, `fromCode`, `next`), `annot.Level`; ordinal switch `Nested.enumSwitch` via synthetic `Nested$1.$SwitchMap$fixtures$Nested$Color` |
| generics (Signature) | `Nested.accept`, `Nested$Visitor`, `Nested$Box`, `Nested.generic`, `Nested.maxKey`, `annot.Annotated.max` |
| bridge methods (ACC_BRIDGE, 5 total) | javac bridges: `Nested$StringVisitor.visit(Nested)Object`, `Nested$Comparer.compareTo(Object)`, `Nested$2.compareTo(Object)`; d8 nest-access accessors (BRIDGE\|SYNTHETIC): `Nested.-$$Nest$fgetouterValue`, `Nested.-$$Nest$fputouterValue` (from `Nested$Inner`/`Nested$1` touching private `outerValue`) |
| native (ACC_NATIVE) | `Nested.nativeMethod(I)I`, `Nested.nativeStatic(Ljava/lang/String;J)V`, `Nested.nativeWide(DJ)D` |
| varargs (ACC_VARARGS) | `Nested.varargs(int, int...)`, `Nested.varargsObj(String, Object...)` |
| default / static interface methods | `Invokes$Greeter.hello()`, `Invokes$Greeter.none()` |
| static / instance init blocks | `Nested.<clinit>` (`staticInit`), `Nested.<init>` (`instanceInit`) |
| class with >300 methods | `large.ManyMethods` (350 methods `m000`..`m349`, mixed static/instance/private/protected) |
| >16 registers, move/from16, move-wide/from16, move-object/from16 | `large.Large.manyLocals(int)` (98 registers; 40 int + 12 long + misc named locals), `large.Large.consume` (15 params) |
| registers >= 256, move/16 | `large.Large.hugeLocals(int)` (304 registers, 4325 code units; 355 `move/16`) |
| several-thousand-instruction straight-line method | `large.Large.linearLong(long,int)` (4600 statements, 33727 16-bit code units) and `large.Large.loopLinear(int,boolean)` (33743 code units, loop back-edge is `goto/32`) |

### Annotations (package `fixtures.annot`)
| Feature | Where |
| --- | --- |
| RUNTIME `@interface` with int, String, Class, enum, int[], String[], nested annotation, long, double, float, boolean, char, byte, short, Class[], enum[] elements, all with defaults (`AnnotationDefault`) | `annot.Marker` |
| nested annotation type | `annot.Inner` (used inside `Marker.inner()`), `annot.Annotated$NestedAnno` |
| CLASS retention (visibility BUILD) | `annot.BuildOnly`, applied to `Annotated` class, `Annotated.field`, `Annotated.method` |
| SOURCE retention (must be absent) | `annot.SourceOnly` -- the type itself is compiled, its application on `Annotated` is not in the DEX |
| class annotation | `Annotated` (`@Marker(...)` with every element set, `@BuildOnly`) |
| field annotation | `Annotated.field` (`@Marker`, `@BuildOnly`), `Annotated.CONST` (`@Inner`), `Annotated.deprecatedField` |
| method annotation | `Annotated.method`, `Annotated.deprecated` (`java.lang.Deprecated`), `Annotated.nestedAnnotated` |
| constructor annotation | `Annotated.<init>` (`@Marker(id = 5)`) |
| parameter annotations (annotation_set_ref_list) | `Annotated.method(@Marker int, @Inner String, int)`, `Annotated.annotatedParamsOnly` |
| system annotations | `Signature`, `Throws` (`Annotated.max`), `EnclosingMethod`, `EnclosingClass`, `InnerClass`, `MemberClasses`, `MethodParameters` (from `-parameters`), `AnnotationDefault` |

### Reflection
`Reflect.call` (`Class.forName("fixtures.Target")`, `getDeclaredMethod("secret")`, `setAccessible`,
`Method.invoke`), `Reflect.callWithArgs` (`getDeclaredMethod("add", int.class, int.class)`,
`invoke(null, 1, 2)`), `Reflect.fieldValue` (`getDeclaredField("hidden")`), `Reflect.loadByLoader`
(3-arg `Class.forName`), `Reflect.viaClassLiteral`. Target: `fixtures.Target` (private `secret()`,
static `add`, private field `hidden`).

### Debug info
Debug builds: every method has `positions` (line table) and `locals` with names/types, e.g.
`Arith.intLit8` (local `r`), `Large.manyLocals` (176 local records incl. re-definitions),
`Exceptions.*` (exception locals `e`, `t`). Parameter names are in `MethodParameters`
annotations and in the debug stream. `release.dex` keeps a reduced line table and parameter
entries only (see header-counts note above).

### DEX 038 build specifics (`v038.dex`)
Compiled with `--no-desugaring`, so the `fixtures.indy.*` classes keep their JVM-level
constructs: 17 `call_site_id`s, 14 `method_handle`s, 16 `invoke-custom` + 1 `invoke-custom/range`,
6 `invoke-polymorphic` + 1 `invoke-polymorphic/range`. No desugared lambda classes exist, so it
has fewer class_defs (49) than the desugared builds (62-64). Classes present only in the
desugared builds: `fixtures.indy.Lambdas$0` .. `Lambdas$11` (lambda/method-ref implementations),
`com.android.tools.r8.annotations.LambdaMethod` (marker annotation d8 puts on them),
`fixtures.indy.Handles$0` (the invoke-polymorphic thrower, v035/v037/release only), and
`fixtures.Invokes$Greeter$-CC` (default-method companion class, v035/release only since
`--min-api 24` supports default methods natively).

## What the other builds do with the indy sources
* `v035`/`v037`/`release` (`--min-api 21/24`): lambdas and method refs become
  `fixtures.indy.Lambdas$N` classes (invoke-static/new-instance), string concat becomes
  `StringBuilder` chains. `MethodHandle.invokeExact/invoke` cannot be desugared; d8 prints a
  warning and replaces each call with `const-string "Instruction is unrepresentable in DEX V35:
  invoke-polymorphic"` + a call to a synthetic `fixtures.indy.Handles$0.m(String)RuntimeException`
  thrower. The surrounding `lookup().findStatic(...)` code is intact.
* `v039` (`--min-api 28`, desugaring on): lambdas/concat are desugared (0 `invoke-custom`), but
  `invoke-polymorphic` is kept natively (6 + 1 `/range`, API >= 26).

## Not produced (and why)
* `const-string/jumbo`: needs > 65,536 string_ids; would require a multi-megabyte generated source.
* `const-method-handle` / `const-method-type` (039): javac only emits `ldc` of a MethodHandle/MethodType
  through constant-dynamic/bootstrap arguments, never for plain Java expressions; d8 therefore
  never produces these from javac output (`indy.Handles.type()` builds a `MethodType` via a call instead).
* `invoke-custom` in `v039.dex`: default desugaring removes all invokedynamic at any API level.
  Use `v038.dex` for call sites.
* `goto/32` in `release.dex`: the release block layout keeps all branch offsets within 16 bits.
* `SourceOnly` annotation *applications*: correctly absent (SOURCE retention).
