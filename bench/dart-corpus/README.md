# dart-corpus

A regression of the Dart extractor on real code. The project here imports 55 popular pure-Dart packages
(`pubspec.yaml`, `lib/all.dart`); the extractor reads what a project imports and what that imports, so
extracting it puts the code of the 55 packages and of the packages they use (88 in the lock file) through
the extractor.

```sh
cd bench/dart-corpus && dart pub get && ./run.sh        # about 5 minutes; needs `rivus` on the PATH
```

It fails when a construct is left out (`unsupported` in the IR) or when the stream is not a valid IR (a function
defined twice, for example). Measured at the commit that added it: 317 MB of IR, no construct left out, a valid
IR. The extractor's own defects found on real code so far (nested collection elements, null-aware elements,
a constructor and a method of the same name, unnamed extensions) were found on `dart-lang/pub` and its
dependencies (`bench/dart-pub`); the 55 packages here found nothing new, which is the point of keeping it:
the next change of the extractor is checked against them.

The list of packages is a sample, not a claim of coverage. `pubspec.lock` is not kept: a newer version of a
package can bring new syntax, which is a reason to run it again.
