// Level definitions for the package, copy of Golang's slog package.
export enum Level {
  debug,
  info,
  warn,
  error,
}

export const Levels: Record<string, Level> = {
  warn: Level.warn,
  debug: Level.debug,
  dbug: Level.debug,
  warning: Level.warn,
  info: Level.info,
  eror: Level.error,
  error: Level.error,
}

export enum Format {
  text,
  json,
}

export const Formats: Record<string, Format> = {
  text: Format.text,
  json: Format.json,
}

enum Color {
  reset = 0,
  black = 30,
  red = 31,
  green = 32,
  yellow = 33,
  blue = 34,
  magenta = 35,
  cyan = 36,
  white = 37,
}

export class Logger {
  keyvals: any[]
  level: Level
  format: Format

  constructor(level: Level, format: Format, ...keyvals: any[]) {
    if (keyvals.length % 2 != 0) {
      keyvals.push("N/A")
    }
    this.keyvals = keyvals
    this.level = level
    this.format = format
  }

  public log(level: Level, msg: string, ...keyvals: any[]) {
    if (level < this.level) {
      return
    }

    if (keyvals.length % 2 != 0) {
      keyvals.push("N/A")
    }

    const r: { [key: string]: string } = {}

    for (let i = 0; i < this.keyvals.length; i += 2) {
      r[this.keyvals[i]] = this.keyvals[i + 1]
    }

    for (let i = 0; i < keyvals.length; i += 2) {
      r[keyvals[i]] = keyvals[i + 1]
    }

    const now = new Date()
    switch (this.format) {
      case Format.json:
        r["msg"] = msg
        r["lvl"] = Level[level]
        r["t"] = now.toJSON()
        console.log(JSON.stringify(r))
        break

      case Format.text:
        var color: Color
        switch (level) {
          case Level.debug:
            color = Color.blue
            break
          case Level.info:
            color = Color.green
            break
          case Level.warn:
            color = Color.yellow
            break
          case Level.error:
            color = Color.red
            break
        }

        let record: string = ""
        record += format("15:04:05.000", now)

        record += " "
        record += "\x1b[" + color + "m"
        record += Level[level].toUpperCase().padEnd(5, " ")
        record += "\x1b[" + Color.reset + "m"

        record += " "
        record += msg.padEnd(25, " ")

        delete r["app"]
        delete r["version"]

        for (const key in r) {
          record += " "
          record += "\x1b[" + color + "m"
          record += key
          record += "\x1b[" + Color.reset + "m"
          record += "="
          record += r[key]
        }

        console.log(record)
        break
    }
  }

  public error(msg: string, ...keyvals: any[]) {
    this.log(Level.error, msg, ...keyvals)
  }

  public warn(msg: string, ...keyvals: any[]) {
    this.log(Level.warn, msg, ...keyvals)
  }

  public info(msg: string, ...keyvals: any[]) {
    this.log(Level.info, msg, ...keyvals)
  }

  public debug(msg: string, ...keyvals: any[]) {
    this.log(Level.debug, msg, ...keyvals)
  }

  public with(...keyvals: any[]) {
    return new Logger(this.level, this.format, ...[...this.keyvals, ...keyvals])
  }
}

function format(fmt: string, d: Date): string {
  return fmt
    .replace("15", d.getHours().toString())
    .replace("04", d.getMinutes().toString())
    .replace("05", d.getSeconds().toString())
    .replace("000", d.getMilliseconds().toString())
}
