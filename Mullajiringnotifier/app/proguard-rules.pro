# Keep the serializable DTOs and their generated serializers (kotlinx.serialization).
-keepattributes *Annotation*, InnerClasses, Signature, RuntimeVisibleAnnotations
-keepclassmembers @kotlinx.serialization.Serializable class uz.kripton.mullajiring.notifier.** {
    *** Companion;
    *** INSTANCE;
}
-keep,includedescriptorclasses class uz.kripton.mullajiring.notifier.**$$serializer { *; }
-keep class uz.kripton.mullajiring.notifier.net.** { *; }

# Tink (security-crypto) references compile-only errorprone annotations not on the runtime classpath.
-dontwarn com.google.errorprone.annotations.**

# Room, OkHttp, WorkManager and Tink ship their own consumer rules; nothing extra needed here.
