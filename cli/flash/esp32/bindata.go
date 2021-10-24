// Code generated for package esp32 by go-bindata DO NOT EDIT. (@generated)
// sources:
// stub/stub.json
package esp32

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

var _stubStubJson = []byte(`{"params_start": 1074331648, "code": "90e2fa3f1020f43f0020f43f00000008b02106403661000c18890181f9ff8208018c480c52460c0000a1f6ffb1f7ffd1f7ff9d08c02000926a00c02000d26b00c02000880b5678ffc02000880ac852c08810890107e8dc860300bd02ad0181ecffe00800880107e8f0280129030c021df0000000f820f43ff830f43f36610081fdffc0200038083030245643ff81faffc020003228003030245633ff10b12020a22025f7ff0c12a023831df070e2fa3f000000403661000c0222610021fcff20a220a5fbff81d0ff31faffc020003908c0200038085673ff0c234602000010b12020a220e5f2ff8221008703f00c021df0000000ffffff00000000200420f43f8020f43f000000023641004060148c360c12862b0051e5ff98459082e24a888739edc62300e5f8ff564afe91f2ffb1f3ff9082101cf947a92b91efffc1f1ff908820c02000890b0c8876880caab3b80bca9ac02000b9094baa32c32042c4e022c220c60e00809401808920c02000890b4082744040148c341b8880807491e0ff06040000e04a119ab44a4348041baac02000490ba040748734e9e088118a334d068199ff91d8ffc020009908c020009228005669ff50a52025ecffa61402c6d8ff22a0001df00000364100a1b8ffe5eaffb2a03530a320818effe008008223002d0a80881189031df000000000000004364100ad0265e8ff4d022182ff8182ffc02000390221f9ffc020002908c020002228005662ff40a42025e6ff900000000820f43f0000400036610031a0ff0c12ad03a5e4ffbd01ad03e5f8ff568a04280142a200402210ad03290125e3ff216eff4197ffc020004902c020004222005664ff30a32065e1ff21eeff91eeffc020008802ad03908820c020008902b80122a001e5f6ffa024831df0000036410065e2ff22a001560a023185ff30a320e5ddff8159ff21e0ffc020002908c0200028085672ffad0365dcff1df0001c20f43fffffffdf2020f43fffffff030000005c36610051faff8d0261faffc020002805602210c02000290551f7ff61f7ffc02000280560221061f5ff602220c020002905216aff8a5462220157b6040c12c6180028422058e25072c077b409cd04bd03ad084611000080a82070c72030b320890165ddff8801562afd2055c04a552055c26d0a06050020c220826100a5dbff8801567afb1b662a7760607470b241e0bb118aa7bab35736dd70c4c0e5d9ff0c130c02a023931df00000200001fffffff7000000700000007c2420f43feb00007000000050bb0000700000ffff6b0000703b0000700b000070030000702c20f43f00000400364100519affc1efffc02000a80581beffc05a10b1edff910dffc7955ac02000a80851eaffb0aa10c02000a908c02000a80861e9ff50aa20c02000a908a1b4ff51b5ffc02000880a50881051e1ff508820c02000890a820901c02000b80a3b8892af00808074909b10908820c02000890a51d8ff46690000d7ea02464600c02000c80851d6ffb0bc10c02000b908c02000b80850bb20c02000b90877fa02c62400520901a19bffecd5c0200098085197ff61cbff509910c020009908c02000880a5194ff50881051c2ff508820c02000890a51c0ff065100c02000b8085119ff50bb20c02000b908c02000880a5189ff50881051b7ff508820c02000890a820901c02000b80a0b8892af00909b1080807490882091b0ffc02000890ac02000880951b0ff50881052a0bb508820c020008909063b0081a7ff51acff47fa0851abffe7ea0251abffa170ffc020005908c02000880a51fcfe508820c02000890aa16cff516cffc02000880a508810516aff508820c02000890a820901c02000b80a7b8892af00808074909b10908820c02000890ac622000000c02000a808b0aa10c02000a908520901dc15c0200098085157ff509910c020009908460b00c02000a80851e0feb152ff50aa20c02000a908820901c02000a80b0b8892af00808074909a10908820c02000890b9147ff517bffc020008809617fff508820c0200089099143ff5144ffc0200088095088105142ff508820516fffc020008909c020006905a1b5fe2a84581a0c1987b502c62a0065a9ff9188fe3cfeb16fffc2a1ffd1c2fefd09462300000080821147ae3dc02000c90b516affc02000890dc020005909517cfec0200088055678ff1c0a768a0f61b6fe6a58c0200068058a5369054b8832c34042c4c022c240c61100c02000890d215affc02000c90bc02000226900c02000280f5672ff4082744040148c341b8880807421a5fee088113a883022c0c60200003a42c02000480449034b338793f1460100a6140246dbff0c092d091df000000000e00000f43f0000f0ffff00cc90004036410081fcffad028a8200881121f8fff62a010c0282c8142a88c02000980821f6ffc08911208810909435902820303282a0a07440b3c281f1ffe008001df0000080fd3fff0f000036410081fdffa2d840b8aa709b11ba99d099119a889818b1f9ff973b0e1bb9b9189a8822480888ba1b8889ba1df000000800f43f0000f43f30f0fd3f081000000c00f43f1c00f43f1000f43f36410021f8ffc02000480221e8ff52d24006370000000c0681f4ffc02000a8088802a09074b65802862e00b1f0ffe0c811cabbb80ba00b0082a0c08719020629000c1889020c0889b546260000b2a0dbb799040c38c62200c2a0c0b8a5c799301b8b2648024600000c0889a598a5a1e0ff7089119a88d088118a820c099918aa8892480088b5979802060d009902061500661814708b11ba88d08811a1d5ff8a82aa88924800060e00a0a07465f0ffc60c0000000082a0dc87191982a0dd87191a0c43390221cbffc020003802c020003902460a00a2a0c0c6000000a2a0db65edff0c2889021b6660607467130286c9ff61c1ffc0200038063030745663f121bfffc0200049021df000000058100000701000007cda0540409300409cda05401cdb054036412152d110ad0581faffe008000c06461800006073c0407763cd07bd012aa625b3ff8d0a561a0491f1ffbd071a998909ad0181f0ffe0080091ecff0c4b1a99880982651681e8ff8aa1a57000664a1fc22516c7b61e60ccc0bd0150a52081e6ffe008006225168602005c32060900005c4286070037369ba1dcffbd051aaa81dfffe00800a1d8ff1c0b1aaa81daffe008000c021df00000364100a1ecfde577ff916efea16ffec020008809a08810c020008909916cfea16cfec020008809a08810a16afea08820c02000890981b6fd91e0fdc020009908c0200098085679ffa1effdc02000290ac0200039082d091df00000000000fd3f3040000000800000440709402400f43f64000094ffff00000000800000000100000000014cc40040ec6700400868004050000640c8c20040fc670040366102a2c15822613c42613f81acffe0080042213c3c224040b456442a3040b43c3256c429e586ff4d0a3c42562a295149ffc1e4ffbd0a50a52081ebffe00800c1e2ffa1dfff40b42081e7ffe00800b1dfffcd050c5a81e5ffe0080021ddff72a101c0200068022c0a62613d61daffc020006902614affc02000222600702220c02000290681daffe00800a2a0b0bd042c4caaa122a0d481d4ffe008002a211c8cbd04ad0281d0ffe0080062a0f06a611c8bad02691265550060ea03626130066e00000060ea03460d00000081c3ff4073c0a2213c77b818a070f4dc27b1bfff25e8ff569a1771beff7a44460400000000b1bcff101120e5e6ff567a1642d41082213f8c68780272d7107734be80ea0372212f6067c08a6662612f70ea0362d51082d5208858683692d5408a6682d5308878f8998a6688158a66626201622500c0200088a9f798076646f4c646000000664602c6440080ea0362212c7066c08a6662612c60ea0362613e706f11fa76d07711a109ff8b877a75d817aa777207008a85076727718effb190ff0c1ecd08ad07926143f261428197ffe00800dd0a922143f22142660a020631008d07a0ea0372212db2213ecd0db077c0aa7780b820a2c15872612d826140926143d26141f26142813cffe00800a0ea0370ea03b2212fba77a077c072612f70ea03d22141822140a2213ccd0dbd082574ff922143d22141f2214256ca06b0ea03fa6682212ed066116a657078c0a9166899ba7772612e1b662646056999c6000000a2690962213cc2a058d0668062613c6802cab1dad6ad01d902816cffe00800bd018ba2811effe008001c8bad02253b00c60600003c524612003c620611003c72c60f00004c02460e003c82060d000062220037b602c68fff22a0b0102280b2c158a2c214810effe0080020ea03322130a2a0b03022c0b2a0241aaa2261302536000c023148ff42213d2c0ac0200049038150ffe008001df00010000036410091feffad02bd03cd045c2247390425bfff2d0a1df0b010000036c12161f7ff8ca452a06247b60286240040642051effe5a51ad0581effee00800461700a2d11081ecfee0080060736370c72010b12020a2202570ff8c6a52a063c617000000cd07bd01ad0581e5fee00800ac74cd07bd01a2d11081e1fee00800a1e6ffb2d11010aa8081defee00800a1e3ff1c0b1aaa81d9fee008007a227033c056e3f9b1d2fea1ddffbab11aaa81d5fee00800a1d9ff1c0b1aaa81d0fee008005d032d051df00030f43f000000108030f43f3661000c02290191fbff21fbffc0200029098d02c0200028098022105642ff81f7ff91e9fcc0200088080c4b908810ad01890181bdfee008001df0002000f43f00000200fffffdff44f0fd3f36810022a05531faff224115c02000280381f8ff0c1b802220c020002903c02000280381f5ffa2c115802210c020002903a52000261a02463e008201157cf20b882241148080740c9287b20206370021ebffe088118a822808a002001c0bad01a51d004c1226aa02462f00c821b811a801a5baff460600b2a01010a120e51b005c1226aa02062800c821b811a80165e5ffa241140625001c0bad01251a0022a06126aa02462000c821b811a80125e5ff46f7ff25f0ffc6f5ff00a545ff06f4ff0c020c1ba2c114224114e51500220115c61b00000000b2a01010a120e51500a0ba20ad01460700b2a01010a120e5140022af91664a322801bd0ac020002802a2c110226104251200060600b2a01010a120a5120022afa1668a0e2221008811c0200089020c02c6ffff2241140c1ba2c114650f0022011582c2fa808074b6280206b2ff1df0000000000009401027000050c300007c680040588600406c2a064038320640348500404c82004036610021f6ff0c0a4802381281f7ffe0080081f6ffe008000c0b81f5ffe00800f19afed154ffc19afeb19afee2a1000c0a0c0281f0ffe008002901271315a1e8ff81edffe00800cd03bd0420a2206577ffa26100a1e4ff81e8ffe00800b2a00410a120a50500e5e2ffa02a20a1ddff81e2ffe0080026620581e1ffe008001df030c0fd3f0000fd3f2cf0fd3f30c0fd3f364100a1fcffc1faff0c0ba0ccc0c0c221817dfee0080011f8ff65f5ff91f6ff82a0ff890991f5ff89091df0364100bd03ad02812bfee008001df000a49200403641006d0222a0c081fdffe008005d0a279af40c0272a0c0460c00000081f7ffe008004d0a771a3882a0db879a1781f3ffe0080082a0dc871a0982a0dd871a05c60300004d052a864248001b223792cc4600000c0232a0c081e9ffe00800379af61df000", "code_start": 1074331656, "entry": 1074335612, "data": "7c0709409107094091070940f907094043080940dc0d0940f70d0940170e0940330e09403a0e0940400e0940400e0940560e0940670e09408b0e0940", "data_start": 1073606704, "num_params": 2, "code_size": 4120, "data_size": 60}`)

func stubStubJsonBytes() ([]byte, error) {
	return _stubStubJson, nil
}

func stubStubJson() (*asset, error) {
	bytes, err := stubStubJsonBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "stub/stub.json", size: 8538, mode: os.FileMode(420), modTime: time.Unix(1, 0)}
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
