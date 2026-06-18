package uz.kripton.mullajiring.notifier.data

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.TypeConverter
import androidx.room.TypeConverters

class Converters {
    @TypeConverter
    fun toStatus(value: String): OutboxStatus = OutboxStatus.valueOf(value)

    @TypeConverter
    fun fromStatus(status: OutboxStatus): String = status.name
}

@Database(
    entities = [OutboxEntity::class, ParseFailureEntity::class, CardBalanceEntity::class],
    version = 1,
    exportSchema = false,
)
@TypeConverters(Converters::class)
abstract class NotifierDatabase : RoomDatabase() {
    abstract fun outboxDao(): OutboxDao
    abstract fun parseFailureDao(): ParseFailureDao
    abstract fun cardBalanceDao(): CardBalanceDao

    companion object {
        @Volatile
        private var instance: NotifierDatabase? = null

        fun get(context: Context): NotifierDatabase = instance ?: synchronized(this) {
            instance ?: Room.databaseBuilder(
                context.applicationContext,
                NotifierDatabase::class.java,
                "notifier.db",
            ).build().also { instance = it }
        }
    }
}
