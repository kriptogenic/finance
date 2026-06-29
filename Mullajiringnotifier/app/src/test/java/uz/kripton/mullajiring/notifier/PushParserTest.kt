package uz.kripton.mullajiring.notifier

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import uz.kripton.mullajiring.notifier.parser.ParseResult
import uz.kripton.mullajiring.notifier.parser.PushParser
import uz.kripton.mullajiring.notifier.parser.TransactionType
import org.junit.Test

class PushParserTest {

    private fun success(body: String) =
        (PushParser.parse(body) as ParseResult.Success).transaction

    // Exactly what the notification listener assembles for the img.png example.
    private val sdMart = listOf(
        "Оплата по карте Visa *2217",
        "Сумма операции: 21 000 UZS",
        "Доступно: 101 979 UZS",
        "Магазин: ООО SD MART",
        "Место: Toshkent, UZ",
        "Дата: 2026-06-26 11:28:49",
    ).joinToString("\n")

    @Test
    fun `ipak yuli card purchase`() {
        val tx = success(sdMart)
        assertEquals(TransactionType.EXPENSE, tx.type)
        assertEquals("ООО SD MART", tx.merchant)
        assertEquals(2_100_000L, tx.amountMinor)
        assertEquals("UZS", tx.currency)
        assertEquals(10_197_900L, tx.balanceMinor)
        assertEquals("UZS", tx.balanceCurrency)
        assertEquals("2217", tx.cardLast4)
        // 11:28:49 Asia/Tashkent (UTC+5) -> 06:28:49Z
        assertEquals("2026-06-26T06:28:49Z", tx.occurredAtUtc)
    }

    @Test
    fun `title and body arrive as separate lines`() {
        // Listener may put the headline in EXTRA_TITLE and the rest in EXTRA_BIG_TEXT.
        val tx = success("Оплата по карте Visa *2217\n$sdMart")
        assertEquals("2217", tx.cardLast4)
        assertEquals(2_100_000L, tx.amountMinor)
    }

    @Test
    fun `narrow no-break space group separator is handled`() {
        val nnbsp = '\u202F' // narrow no-break space, as some banks emit
        val tx = success(
            "Оплата по карте Visa *2217\n" +
                "Сумма операции: 1${nnbsp}234${nnbsp}567 UZS\n" +
                "Магазин: SHOP\n" +
                "Дата: 2026-06-26 11:28:49",
        )
        assertEquals(123_456_700L, tx.amountMinor)
        assertEquals("SHOP", tx.merchant)
    }

    @Test
    fun `OTP push is ignored not failed`() {
        // No "Оплата по карте" marker => never logged or stored.
        assertEquals(ParseResult.Ignored, PushParser.parse("Kod podtverzhdeniya: 123456"))
        assertEquals(ParseResult.Ignored, PushParser.parse("Vhod v prilozhenie vypolnen"))
    }

    @Test
    fun `purchase missing fields is a parse failure`() {
        val result = PushParser.parse("Оплата по карте Visa *2217\nчто-то пошло не так")
        assertTrue(result is ParseResult.Failed)
    }
}
