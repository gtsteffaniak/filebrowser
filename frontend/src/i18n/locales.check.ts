// Checked by `npm run typecheck`
import en from "./locales/en.json" with { type: "json" };
import ar from "./locales/ar.json" with { type: "json" };
import bg from "./locales/bg.json" with { type: "json" };
import cz from "./locales/cz.json" with { type: "json" };
import de from "./locales/de.json" with { type: "json" };
import el from "./locales/el.json" with { type: "json" };
import es from "./locales/es.json" with { type: "json" };
import eu from "./locales/eu.json" with { type: "json" };
import fr from "./locales/fr.json" with { type: "json" };
import he from "./locales/he.json" with { type: "json" };
import hu from "./locales/hu.json" with { type: "json" };
import it from "./locales/it.json" with { type: "json" };
import ja from "./locales/ja.json" with { type: "json" };
import ko from "./locales/ko.json" with { type: "json" };
import nlBe from "./locales/nl-be.json" with { type: "json" };
import nl from "./locales/nl.json" with { type: "json" };
import pl from "./locales/pl.json" with { type: "json" };
import ptBr from "./locales/pt-br.json" with { type: "json" };
import pt from "./locales/pt.json" with { type: "json" };
import ro from "./locales/ro.json" with { type: "json" };
import ru from "./locales/ru.json" with { type: "json" };
import sk from "./locales/sk.json" with { type: "json" };
import svSe from "./locales/sv-se.json" with { type: "json" };
import tr from "./locales/tr.json" with { type: "json" };
import ua from "./locales/ua.json" with { type: "json" };
import zhCn from "./locales/zh-cn.json" with { type: "json" };
import zhTw from "./locales/zh-tw.json" with { type: "json" };

type MessageSchema = typeof en;
type DeepPartial<T> = { [K in keyof T]?: T[K] extends string ? string : DeepPartial<T[K]> };
type NoExtra<T, S> = T extends string ? T : { [K in keyof T]: K extends keyof S ? NoExtra<T[K], S[K]> : never };

const exact = <T extends Record<string, DeepPartial<MessageSchema>>>(
  _locales: T & { [L in keyof T]: NoExtra<T[L], MessageSchema> },
): void => {};

exact({
  ar,
  bg,
  cz,
  de,
  el,
  es,
  eu,
  fr,
  he,
  hu,
  it,
  ja,
  ko,
  "nl-be": nlBe,
  nl,
  pl,
  "pt-br": ptBr,
  pt,
  ro,
  ru,
  sk,
  "sv-se": svSe,
  tr,
  ua,
  "zh-cn": zhCn,
  "zh-tw": zhTw,
});
