// Code generated for package esp32s3 by go-bindata DO NOT EDIT. (@generated)
// sources:
// stub/stub.json
package esp32s3

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)
type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _stubStubJson = []byte(`{"params_start": 1077436416, "code": "60000c6014200060000000804c1a0040741f0040ec0a004036410091f9ffa2ac00c020008809a08810c02000826900a2a02881f6ffe0080081f6ffe008000c0b81f5ffe0080091efffa1efffc020008809a08820c0200089091df00000e00000a806004036410082d260ad0200881121fbfff62a0222a00082c8142a88c020009808a0a074c089118080f49094359a2830328240b3c281f2ffe008001df000000060cb3fff0f000036410081fdffa2d840b8aa709b11ba99d099119a889818b1f9ff973b0e1bb9b9189a8822480888ba1b8889ba1df00000080000600000006030d0cb3f081000000c0000601c0000601000006036410021f8ffc02000480221e8ff52d240063f0000000c0681f4ffc02000a8088802a0b074b6580206370091f0ffe0c811ca999809a0090082a0c0871b028631000c1889020c0889b5c62e000092a0db979b040c38062b00c2a0c098a5c79b531b892648024600000c0889a5a8a50c0b708a11aa88d08811a1deff8a82b918aa88b2480088b5b798237089119a88d088118a82aa88820800808004b7980e0c43390221d4ffc020008902461c000c0886140000006618147089119a88d0881191ccff8a829a88b24800460d00a0a07465eeff860c0000000082a0dc871b1682a0dd871b170c43390221c3ff0c03c020003902460a00a2a0c0c6000000a2a0db65ebff82a00282620062c60167130286c1ff61b9ffc0200038063030945663ef21b7ffc0200049021df000000058100000701000005c1c0040200a004044070040681c0040741c004036412152d110ad0581f9ffe008000c06461900006073c0407763cd07bd012aa681f4ffe008008d0a562a0491efffbd071a998909ad0181efffe0080091ebff0c4b1a99880982651681e7ff8aa1a57800664a20c22516c7b61f60ccc010b12050a52081e5ffe008006225168602005c32060900005c42860700373697a1daffbd051aaa81deffe00800a1d6ff1c0b1aaa81d9ffe008000c021df00000e4ffce3f18200060ffffffdf1c200060ffffff030000005c0020006000000040042000606009004036410081f5ffa80881fdffe0080091f3ffa1f3ffc020008809a08810c02000890991f0ffa1f1ffc020008809a08810a1efffa08820c02000890981edff91edffc020009908c0200098085679ffa1eaffc02000290ac0200039082d091df000000010000000e0ca3f3040000000800000fc603840240000606400a800ffff0000000080000000010000000001000e27072c0a0040e8110040781b0040901b00404c080040140a0040f4110040841b0040368102426141a2c1582040b432613d819affe008003c2356d42f32213d3040b43c3356242f81eeffe008004d0a3c43565a2e612fffc1e0ffbd0aad0681eaffe00800c1deffa1dbffbd0481e6ffe00800b1dcffcd040c5a81e4ffe0080031d9ff2c0ac02000580352613f51d7ffc0200059033131ff52a101c02000590381dbffe00800a2a0b0bd042c4caaa132a0d481d5ffe008003a311c8c40b42030a32081d1ffe008001c8bad03655a0050ea0352613046860050ea03860d000082213d4078c081c2ff77b81a2070f4564701b1c0ff20a220a5e7ff565a1d71beff704480860300b1bdff20a22065e6ff565a1c42d41092214116890072230072d7107734ba80ea0372212f5057c08a5552612fa0ea0372d6107837581688067a5572d6207857b1aeff7a5572d63078775a5772d6405913589752613c51eefe460300000090ea03a099c097bb02465b00c0200098a7c2213cc799056648e586580066480206570090ea0382212ca088c09a8882612c80ea0392213c82614082213c70991192613e8a99d099118b895a889a5591e9fe9a9592090007691b5185ffad08cd05b2a0ff826143818fffe00800dd05822143c6080000d81517691d517effb17fff0c1e80c82050a520818affe00800dd0a660a02863b008d0590ea0352212da22140cd0da055c09a55a2c158bd0852612dd261448261438123ffe0080092213ea2213cd22144aa59d0551191cafe5a569a5552050007e54c90ea035143ffa805926142814affe0080050ea03a2212f922142aa559055c052612f50ea03d22144822143cd0d80b82020a220816affe00800d22144563a0780ea0392212e5059c08a5552612e52213e92213c9a85d0881158978a860c0999181b552645055997c6000000926709522300c2a058da55cab1ad01da2259038158ffe00800bd018ba381fafee008001c8bad03653a00c60700003c53c613003c638612003c73461100003c83c60f003c93860e004c03460d00580382213d87b5028676ff92a0b0109980b2c158a2c91481e8fee0080020ea03322130a2a0b03022c02c4b1aaa22613032a000e53400212fff42213f2c0ac020004902813affe008002d031df03641009124ffad02bd03cd045c22473904e5b5ff2d0a1df0b010000036c121611dff8ca452a06247b60286240040642051c9fe5a51ad0581c9fee00800461700a2d11081c6fee00800607363cd07bd01ad0281c3fee008008c6a52a063c617000000cd07bd01ad0581c0fee00800ac74cd07bd01a2d11081bcfee00800a1e6ffb2d11010aa8081b9fee00800a1e3ff1c0b1aaa81b4fee008007a227033c056e3f9b1acfea1ddffbab11aaa81b0fee00800a1d9ff1c0b1aaa81abfee008005d032d051df00000001058200060ffffff0036410091d4fe81fbffc020008909c0200028098782f721f8ff81f8ffc0200028028022101df0000036610065fdffa26100b2a00410a1208197fee008000c021df00000002000006000000200fffffdff44d0cb3f9c09004036810022a05531f9ff224115c02000280381f7ff0c1b802220c020002903c02000280381f4ffa2c115802210c020002903e51e00261a02463f008201157cf20b882241148080740c9287b20206380021eaffe088118a822808a002001c0bad01e51b004c1226aa02463000c821b811a801a5b4ff460600b2a01010a120251a005c1226aa02062900c821b811a801a5e4ffa241140626001c0bad0165180022a06126aa02462100c821b811a80165e4ff46f7ffe5f1ffc6f5ff0081d0ffe0080046f3ff000c020c1ba2c114224114e51300220115c61b00000000b2a01010a120e51300a0ba20ad01460700b2a01010a120e5120022af91664a322801bd0ac020002802a2c110226104251000060600b2a01010a120a5100022afa1668a0e2221008811c0200089020c02c6ffff2241140c1ba2c114650d0022011582c2fa808074b6280206b1ff1df0000000006038401027000050c3000080070040500a004000060040d806004036610021f8ff0c0a4802381281f9ffe00800a56afff182fed17bfec183feb183fee2a1000c0a0c0281f3ffe008002901271314a1edff81f0ffe00800cd03bd04ad02656cffa26100a1e9ff81ebffe00800b2a00410a120e50400e5e3ffa02a20a1e2ff81e5ffe0080026620581e4ffe008001df030a0cb3f00e0ca3f2cd0cb3f364100a1fdffc1fbff0c0ba0ccc0c0c2218169fee0080081f9ff80182025f6ff1df00000364100bd03ad02810afee008001df0006c0600403641006d0222a0c081fdffe008005d0a279af40c0272a0c0460c00000081f7ffe008004d0a771a3882a0db879a1781f3ffe0080082a0dc871a0982a0dd871a05c60300004d052a864248001b223792cc4600000c0232a0c081e9ffe00800379af61df000", "code_start": 1077436424, "entry": 1077438948, "data": "346138404961384049613840d46138401b623840586838407368384093683840af683840b6683840c0683840c0683840d6683840e76838400b693840", "data_start": 1070321712, "num_params": 2, "code_size": 2680, "data_size": 60}`)

func stubStubJsonBytes() ([]byte, error) {
	return _stubStubJson, nil
}

func stubStubJson() (*asset, error) {
	bytes, err := stubStubJsonBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "stub/stub.json", size: 5658, mode: os.FileMode(420), modTime: time.Unix(1, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"stub/stub.json": stubStubJson,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"stub": &bintree{nil, map[string]*bintree{
		"stub.json": &bintree{stubStubJson, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
