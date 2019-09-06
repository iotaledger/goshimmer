var fs = require('fs');

async function run() {
    var content = 'package ui\n\nvar files = map[string]string{\n'
    var files = walk('.')
    var skip = ['favicon.ico','builder.js']
    await asyncForEach(files, async function(file){
        var fileName = file.substring(2, file.length)
        if(skip.includes(fileName)) return
        var data = await readFile(file)
        content += addContent(fileName, data)
    })
    content += '\n}\n'
    fs.writeFile('../files.go',content,function(){
        console.log('files.go created!')
    })
}

function addContent(fileName, data) {
    return '\t"'+fileName+'":`' +data +'`,\n'
}

function readFile(file) {
    return new Promise(function(resolve,reject){
        fs.readFile(file, {encoding: 'utf-8'}, function(err,data){
            if (!err) {
                resolve(data)
            } else {
                reject(err)
            }
        });
    })
}

var walk = function(dir) {
    var results = [];
    var list = fs.readdirSync(dir);
    list.forEach(function(file) {
        file = dir + '/' + file;
        var stat = fs.statSync(file);
        if (stat && stat.isDirectory()) { 
            /* Recurse into a subdirectory */
            results = results.concat(walk(file));
        } else { 
            /* Is a file */
            results.push(file);
        }
    });
    return results;
}

async function asyncForEach(array, callback) {
    for (let index = 0; index < array.length; index++) {
        await callback(array[index], index, array);
    }
}

run()