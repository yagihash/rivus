#!/usr/bin/env bash
# リリース用のアーカイブを dist/ に作る（tagpr.yml の release-binaries と、PR の ci.yml の release-build が使う）。
#
# 使い方: scripts/build-release-archives.sh <版（v なし。例: 1.2.3）>
#
# どのターゲットも cgo を使わない（modernc.org/sqlite は純 Go）。静的リンクで、1 台からまとめてクロスビルドする。
# 各アーカイブには、rivus の LICENSE と、そのターゲットのバイナリにリンクされたもののライセンス文を入れる
# （MIT / BSD / Apache-2.0 は、バイナリの配布にライセンス文と表示を求める）。
#   - モジュール: LICENSE, LICENCE, COPYING, NOTICE, PATENTS で始まるファイル
#   - Go の標準ライブラリ: $(go env GOROOT) の LICENSE と PATENTS
# ライセンス文が見つからないモジュールがあれば、失敗にする（黙って載せずに出すのを避ける）。
set -euo pipefail

ver="${1:?usage: build-release-archives.sh <version without v>}"
goroot="$(go env GOROOT)"

# 公式の配布物は $GOROOT/LICENSE を持つ。Homebrew の Go は、LICENSE を libexec（= GOROOT）の 1 つ上に置く。
go_license=""
for d in "${goroot}" "${goroot}/.."; do
  if [ -f "${d}/LICENSE" ]; then go_license="${d}/LICENSE"; break; fi
done
if [ -z "${go_license}" ]; then
  echo "no LICENSE of Go found under ${goroot}" >&2
  exit 1
fi

rm -rf build dist
mkdir -p dist

# extractor のソースのハッシュ（本体に埋め込む。計算は internal/extractorhash の 1 か所。ADR-126）。ターゲットの GOOS/GOARCH を設定する前に、ホストで求める。
rust_hash="$(go run ./scripts/extractor-hash rust)"
dart_hash="$(go run ./scripts/extractor-hash dart)"

# バイナリにリンクされる依存に、コピーレフトのライセンスのものがあれば、ビルドを止める（ターゲットごとに、リンクされるものは違う）。
# go-licenses の分類（https://github.com/google/licenseclassifier の category）で、次を禁止する:
#   restricted: GPL, LGPL, AGPL など（強いコピーレフト）
#   reciprocal: MPL, EPL, CDDL など（弱いコピーレフト。変更したファイルの公開を求める）
#   forbidden:  ライセンスがない・利用を制限するもの（CC-BY-NC など）
#   unknown:    分類できないもの（コピーレフトかどうか分からないので、通さない）。中身を確かめて問題がなければ、
#               そのパッケージを go-licenses check の --ignore に足す（例外がこのファイルの差分として残る）
# go-licenses は、ターゲットの GOOS/GOARCH でビルドできるものを調べる。ツール自体は、ホストの向きで入れる。
# renovate: datasource=go depName=github.com/google/go-licenses/v2
go_licenses_version=v2.0.1
tools="$(mktemp -d)"
trap 'rm -rf "${tools}"' EXIT
GOBIN="${tools}" go install "github.com/google/go-licenses/v2@${go_licenses_version}"

for target in linux/amd64 linux/arm64 darwin/arm64; do
  export GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0
  name="rivus_${ver}_${GOOS}_${GOARCH}"
  work="build/${name}"
  "${tools}/go-licenses" check ./cmd/rivus --disallowed_types=forbidden,restricted,reciprocal,unknown
  mkdir -p "${work}/THIRD_PARTY_LICENSES/go"
  go build -trimpath -ldflags "-s -X main.versionStamp=${ver} -X main.rustSourceHash=${rust_hash} -X main.dartSourceHash=${dart_hash}" -o "${work}/rivus" ./cmd/rivus
  cp LICENSE "${work}/LICENSE"
  cp "${go_license}" "${work}/THIRD_PARTY_LICENSES/go/LICENSE"
  if [ -f "${goroot}/PATENTS" ]; then cp "${goroot}/PATENTS" "${work}/THIRD_PARTY_LICENSES/go/PATENTS"; fi
  go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}} {{.Dir}}{{end}}{{end}}' ./cmd/rivus | sort -u |
    while read -r mod dir; do
      dest="${work}/THIRD_PARTY_LICENSES/${mod}"
      mkdir -p "${dest}"
      found=0
      for f in "${dir}"/*; do
        case "$(basename "$f")" in
          LICENSE* | LICENCE* | COPYING* | NOTICE* | PATENTS*)
            cp "$f" "${dest}/"
            found=1
            ;;
        esac
      done
      if [ "$found" -eq 0 ]; then
        echo "no license file in ${mod}" >&2
        exit 1
      fi
    done
  tar -C build -czf "dist/${name}.tar.gz" "${name}"
done

(cd dist && sha256sum ./*.tar.gz > "rivus_${ver}_checksums.txt")
cat dist/*_checksums.txt
