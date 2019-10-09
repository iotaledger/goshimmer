import numpy as np
import matplotlib.pyplot as plt

xcol = 0
xlabel = "Time [seconds]"
#xlim = [0, 100]
# xscale = 'log'
xscale = 'linear'

# - - - - parameters unlikely to be changed - - -
ycol = 1  # last column in the csv file
folder = "data/"


def main():
    printLinkAnalysis()
    printConvergenceAnalysis()


def printLinkAnalysis():
    fig = plt.figure()
    filename = folder+'plot_linkAnalysis'
    partPlot("LinkAnalysis", "linkAnalysis", filename, "blue")
    #plt.ylim([0, 1.05])
    plt.xscale(xscale)
    #plt.xlim(xlim)
    plt.xlabel(xlabel)
    plt.ylabel("Probability")
    #plt.yscale('log')
    plt.legend(loc='best')
    plt.savefig(filename+'.eps', format='eps')
    plt.clf()

def printConvergenceAnalysis():
    fig = plt.figure()
    filename = folder+'plot_convAnalysis'
    partPlot("ConvAnalysis", "convAnalysis", filename, "blue")
    #plt.ylim([0, 1.05])
    plt.xscale(xscale)
    #plt.xlim(xlim)
    plt.xlabel(xlabel)
    plt.ylabel("Nodes with 8 neighbors [%]")
    plt.legend(loc='best')
    plt.savefig(filename+'.eps', format='eps')
    plt.clf()


def partPlot(type, file, filename, color):
    x = loadDatafromRow(file, xcol)
    y = loadDatafromRow(file, ycol)
    x, y = sort2vecs(x, y)
    # if style == '':
    #     plt.plot(x, y, label=type)
    # else:
    #     plt.plot(x, y, label=type, marker=style, linestyle='none')
    #plt.plot(x, y, linestyle='dashed', color=color, linewidth=1)
    plt.plot(x, y, color=color, linewidth=1)
    #plt.ylim([0,np.max(y)])
    np.savez(filename+"_"+type, x=x, y=y)


def sort2vecs(x, y):
    i = np.argsort(x)
    x = x[i]
    y = y[i]
    return x, y


def loadDatafromRow(datatype, row):
    try:
        filestr = folder+'result_'+datatype+'.csv'
        f = open(filestr, "r")
        data = np.loadtxt(f, delimiter=",", skiprows=1, usecols=(row))
        return data
    except IOError:
        print(filestr)
        print("File not found.")
        return []


# needs to be at the very end of the file
if __name__ == '__main__':
    main()