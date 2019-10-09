import numpy as np
import matplotlib.pyplot as plt

xcol = 0
xlabel = "Time [seconds]"
xscale = 'linear'

# - - - - parameters unlikely to be changed - - -
ycol = 1 
zcol = 2  # last column in the csv file
folder = "data/"


def main():
    printLinkAnalysis()
    printConvergenceAnalysis()


def printLinkAnalysis():
    fig = plt.figure()
    filename = folder+'plot_linkAnalysis'
    partPlot2("LinkAnalysis", "linkAnalysis", filename, "blue")
    plt.xscale(xscale)
    #plt.xlim(xlim)
    plt.xlabel(xlabel)
    plt.ylabel("Probability")
    #plt.yscale('log')
    plt.legend(loc='best')
    plt.savefig(filename+'.eps', format='eps')
    plt.clf()

def printConvergenceAnalysis(): 
    filename = folder+'plot_convAnalysis'
    partPlot3("ConvAnalysis", "convAnalysis", filename, "blue")


def partPlot2(type, file, filename, color):
    color = 'tab:blue'
    x = loadDatafromRow(file, xcol)
    y = loadDatafromRow(file, ycol)
    x, y = sort2vecs(x, y)
    plt.plot(x, y, color=color, linewidth=1)
    np.savez(filename+"_"+type, x=x, y=y)

def partPlot3(type, file, filename, color):
    x = loadDatafromRow(file, xcol)
    y = loadDatafromRow(file, ycol)
    z = loadDatafromRow(file, zcol)
    x, y, z = sort3vecs(x, y, z)
    
    fig, ax1 = plt.subplots()
    
    color = 'tab:blue'
    ax1.set_xlabel('Time [seconds]')
    ax1.set_ylabel('Nodes with 8 neighbors [%]', color=color)
    ax1.set_ylim([0, 100])
    ax1.plot(x, y, color=color)
    ax1.tick_params(axis='y', labelcolor=color)

    ax2 = ax1.twinx()  # instantiate a second axes that shares the same x-axis

    color = 'tab:red'
    ax2.set_ylabel('Avg # of neighbors', color=color)  # we already handled the x-label with ax1
    ax2.plot(x, z, color=color)
    ax2.tick_params(axis='y', labelcolor=color)

    fig.tight_layout()  # otherwise the right y-label is slightly clipped

    np.savez(filename+"_"+type, x=x, y=y)
    fig.savefig(filename+'.eps', format='eps')
    fig.clf()


def sort2vecs(x, y):
    i = np.argsort(x)
    x = x[i]
    y = y[i]
    return x, y

def sort3vecs(x, y, z):
    i = np.argsort(x)
    x = x[i]
    y = y[i]
    z = z[i]
    return x, y, z

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