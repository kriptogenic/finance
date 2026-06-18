package uz.kripton.mullajiring.notifier

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import uz.kripton.mullajiring.notifier.parser.IdempotencyKey
import uz.kripton.mullajiring.notifier.parser.ParseResult
import uz.kripton.mullajiring.notifier.parser.SmsParser
import uz.kripton.mullajiring.notifier.parser.TransactionType

class SmsParserTest {

    private fun success(body: String) =
        (SmsParser.parse(body) as ParseResult.Success).transaction

    @Test
    fun `yandex go UZS expense`() {
        val tx = success(
            "Pokupka: YANDEX.GO YUNUSOBOD TUM UZB 2026-06-18 10:08:41, karta 404800***7476. " +
                "summa: 28 500.00 UZS, balans: 19 315 740.00 UZS",
        )
        assertEquals(TransactionType.EXPENSE, tx.type)
        assertEquals("YANDEX.GO YUNUSOBOD TUM", tx.merchant)
        assertEquals(2_850_000L, tx.amountMinor)
        assertEquals("UZS", tx.currency)
        assertEquals(1_931_574_000L, tx.balanceMinor)
        assertEquals("7476", tx.cardLast4)
        // 10:08:41 Asia/Tashkent (UTC+5) -> 05:08:41Z
        assertEquals("2026-06-18T05:08:41Z", tx.occurredAtUtc)
    }

    @Test
    fun `anthropic USD expense`() {
        val tx = success(
            "Pokupka: ANTHROPIC +14152360599 USA 2026-06-17 07:39:15, karta 404800***7476. " +
                "summa: 5.60 USD, balans: 19 344 240.00 UZS",
        )
        assertEquals("ANTHROPIC +14152360599", tx.merchant)
        assertEquals(560L, tx.amountMinor)
        assertEquals("USD", tx.currency)
        assertEquals("UZS", tx.balanceCurrency)
    }

    @Test
    fun `humo money transfer expense`() {
        val tx = success(
            "Pokupka: HUMO MONEY TRANSFER Tashkent UZB 2026-06-17 10:33:25, karta 404800***7476. " +
                "summa: 550 000.00 UZS, balans: 19 412 000.00 UZS",
        )
        assertEquals("HUMO MONEY TRANSFER Tashkent", tx.merchant)
        assertEquals(55_000_000L, tx.amountMinor)
    }

    @Test
    fun `nbsp thousands separator is handled`() {
        val tx = success(
            "Pokupka: SHOP Tashkent UZB 2026-06-17 10:33:25, karta 404800***7476. " +
                "summa: 1 234 567.89 UZS, balans: 0.00 UZS",
        )
        assertEquals(123_456_789L, tx.amountMinor)
        assertEquals(0L, tx.balanceMinor)
    }

    @Test
    fun `comma decimal separator is handled`() {
        assertEquals(2_850_000L, SmsParser.parseMinor("28 500,00"))
        assertEquals(560L, SmsParser.parseMinor("5,60"))
    }

    @Test
    fun `OTP message is ignored not failed`() {
        assertEquals(ParseResult.Ignored, SmsParser.parse("Vash kod podtverzhdeniya: 123456"))
    }

    @Test
    fun `malformed purchase is a parse failure`() {
        val result = SmsParser.parse("Pokupka: SOMETHING WENT WRONG here")
        assertTrue(result is ParseResult.Failed)
    }

    @Test
    fun `idempotency key is stable and sender-bound`() {
        val body = "Pokupka: X UZB 2026-06-17 10:33:25, karta 404800***7476. summa: 1.00 UZS, balans: 2.00 UZS"
        val a = IdempotencyKey.of(body, "infinbank")
        val b = IdempotencyKey.of(body, "infinbank")
        val c = IdempotencyKey.of(body, "other")
        assertEquals(a, b)
        assertTrue(a != c)
        assertEquals(64, a.length)
    }
}
